package hydra

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
	"gopkg.in/yaml.v2"
)

const ModuleSpecFilename string = `module.yaml`

var ManifestFilename string = `manifest.yaml`
var Entrypoint string = executil.Env(`HYDRA_ENTRYPOINT`, `app.yaml`)
var Domain = executil.Env(`HYDRA_HOST`, `hydra.local`)
var Environment = executil.Env(`HYDRA_ENV`)
var ID = executil.Env(`HYDRA_ID`)
var Hostname, _ = os.Hostname()
var Client = &http.Client{
	Timeout: 10 * time.Second,
}

func init() {
	os.Setenv(`HYDRA_ENTRYPOINT`, Entrypoint)
	os.Setenv(`HYDRA_HOST`, Domain)
	os.Setenv(`HYDRA_ENV`, Environment)
	os.Setenv(`HYDRA_ID`, ID)
}

type Application struct {
	Module      `yaml:",inline"`
	Root        string    `yaml:"-" json:"root"`
	Manifest    *Manifest `yaml:"manifest,omitempty"`
	OutputDir   string    `yaml:"-" json:"-"`
	PreserveDir bool      `yaml:"-" json:"-"`
	filename    string
}

func IsLoadErr(err error) bool {
	if err != nil {
		return strings.HasPrefix(err.Error(), `from-`)
	}

	return false
}

// Loads an application from the given io.Reader data.
func FromReader(reader io.Reader) (*Application, error) {
	if data, err := ioutil.ReadAll(reader); err == nil {
		var app Application

		if err := yaml.UnmarshalStrict(data, &app); err == nil {
			app.Name = strings.TrimSuffix(filepath.Base(Entrypoint), filepath.Ext(Entrypoint))

			return &app, nil
		} else {
			return nil, fmt.Errorf("parse: %v", err)
		}
	} else {
		return nil, fmt.Errorf("from-read: %v", err)
	}
}

// Loads an application from the given YAML filename.
func FromFile(yamlFilename string) (*Application, error) {
	if fn, err := fileutil.ExpandUser(yamlFilename); err == nil {
		if file, err := os.Open(fn); err == nil {
			defer file.Close()

			if app, err := FromReader(file); err == nil {
				app.filename = yamlFilename
				app.Root = filepath.Dir(yamlFilename)
				return app, nil
			} else {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("from-open: %v", err)
		}
	} else {
		return nil, fmt.Errorf("from-path: %v", err)
	}
}

// Loads an application from the given URL.
func FromURL(manifestOrAppFileUrl string) (*Application, error) {
	if u, err := url.Parse(manifestOrAppFileUrl); err == nil {
		if res, err := Client.Get(u.String()); err == nil {
			defer res.Body.Close()

			if res.StatusCode < 400 {
				if app, err := FromReader(res.Body); err == nil {
					u.Path = filepath.Dir(u.Path)
					app.Root = u.String()
					return app, nil
				} else {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("from-http: %s", res.Status)
			}
		} else {
			return nil, fmt.Errorf("from-http: %v", err)
		}
	} else {
		return nil, fmt.Errorf("from-url: %v", err)
	}
}

// Automatically attempts to load an application from a variety of sources.  If location is
// a path to a non-empty, readable file, that will be parsed.  If location is a URL, it will
// be retrieved via an HTTP GET request.  If location is empty, the following auto-generated
// locations will be attempted (in order):
//
//   {HYDRA_ENV}.app.yaml
//   app.yaml
//   ~/.config/hydra/{HYDRA_ENV}.app.yaml
//   ~/.config/hydra/app.yaml
//   /etc/hydra/{HYDRA_ENV}.app.yaml
//   /etc/hydra/app.yaml
//   https://{HYDRA_HOST:-hydra.local}/app.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//   http://{HYDRA_HOST:-hydra.local}/app.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//
func Load(locations ...string) (*Application, error) {
	locations = sliceutil.CompactString(locations)

	var candidates []string

	if len(locations) == 0 {
		hasEnv := (Environment != ``)

		if hasEnv {
			candidates = append(candidates, Environment+`.`+ManifestFilename)
			candidates = append(candidates, Environment+`.`+Entrypoint)
		}

		candidates = append(candidates, ManifestFilename)
		candidates = append(candidates, Entrypoint)

		if hasEnv {
			candidates = append(candidates, `~/.config/hydra/`+Environment+`.`+ManifestFilename)
			candidates = append(candidates, `~/.config/hydra/`+Environment+`.`+Entrypoint)
		}

		candidates = append(candidates, `~/.config/hydra/`+ManifestFilename)
		candidates = append(candidates, `~/.config/hydra/`+Entrypoint)

		if hasEnv {
			candidates = append(candidates, `/etc/hydra/`+Environment+`.`+ManifestFilename)
			candidates = append(candidates, `/etc/hydra/`+Environment+`.`+Entrypoint)
		}

		candidates = append(candidates, `/etc/hydra/`+ManifestFilename)
		candidates = append(candidates, `/etc/hydra/`+Entrypoint)

		for _, entrypoint := range []string{
			ManifestFilename,
			Entrypoint,
		} {
			for _, scheme := range []string{
				`https`,
				`http`,
			} {
				candidates = append(candidates, fmt.Sprintf(
					"%s://%s/%s?env=%s&id=%s&host=%s",
					scheme,
					Domain,
					entrypoint,
					Environment,
					ID,
					Hostname,
				))
			}
		}
	} else {
		candidates = locations
	}

	for _, location := range candidates {
		var app *Application
		var err error
		scheme, _ := stringutil.SplitPair(location, `://`)
		log.Debugf("Load: trying %s", location)

		switch scheme {
		case `https`, `http`:
			app, err = FromURL(location)
		default:
			app, err = FromFile(location)
		}

		if err == nil {
			return app, nil
		} else {
			if !IsLoadErr(err) {
				return app, err
			}
		}
	}

	return nil, fmt.Errorf("no application found by any means")
}

func (self *Application) QML() ([]byte, error) {
	var out bytes.Buffer

	// add standard library functions
	self.Modules = append(self.getBuiltinModules(), self.Modules...)

	// process all top-level import statements
	for _, imp := range self.Imports {
		if stmt, err := toImportStatement(imp); err == nil {
			out.WriteString(stmt + "\n")
		} else {
			return nil, err
		}
	}

	// retrieve and write out all modules
	if err := self.writeModules(self, self.OutputDir); err == nil {
		out.WriteString(fmt.Sprintf("import %q\n", `.`))
	} else {
		return nil, err
	}

	out.WriteString("\n")

	if root := self.Definition; root != nil {
		if root.ID == `` {
			root.ID = `root`
		}

		// do some horrors to expose the top-level application item to the stdlib
		var onCompleted string

		if root.Properties == nil {
			root.Properties = make(map[string]interface{})
		}

		if oc, ok := root.Properties[`Component.onCompleted`]; ok {
			onCompleted = typeutil.String(oc) + "\n"
		}

		onCompleted = `Hydra.root = ` + root.ID + ";\n" + onCompleted
		root.Properties[`Component.onCompleted`] = Literal(onCompleted)

		if data, err := root.QML(0, root); err == nil {
			out.Write(data)

			return out.Bytes(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid root definition")
	}
}

func (self *Application) WriteQmlManifest() error {
	if err := filepath.Walk(self.OutputDir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			if info.IsDir() {
				return writeQmldir(path, ``)
			} else {
				return nil
			}
		} else {
			return err
		}
	}); err == nil {
		return writeQmldir(self.OutputDir, `Application`)
	} else {
		return err
	}
}

func (self *Application) String() string {
	if data, err := self.QML(); err == nil {
		return string(data)
	} else {
		return ``
	}
}

func (self *Application) GlobalImportPaths() (paths []string) {
	modpaths := make(map[string]interface{})

	for _, mod := range self.deepSubmodules() {
		if spec := mod.spec; spec != nil && spec.Global {
			dir := filepath.Dir(mod.AbsolutePath(self.OutputDir))
			modpaths[dir] = true
		}
	}

	for _, abs := range maputil.StringKeys(modpaths) {
		paths = append(paths, abs)
	}

	sort.Strings(paths)
	return
}

// generates a syntactically-correct QML import statement from a string.
// 	format: [ALIAS:]MODULE[ MAJOR.MINOR]
//
//  Examples:
//
//		QtQuick 2.0        -> import QtQuick 2.0
//		Q:QtQuick 2.0      -> import QtQuick 2.0 as Q
//		Something.js       -> import "Something.js" as Something
//		Other:Something.js -> import "Something.js" as Other
//
func toImportStatement(imp string) (string, error) {
	imp = strings.TrimSpace(imp)
	imp = env(imp)

	parts := rxutil.Whitespace.Split(imp, 2)
	alias, lib := stringutil.SplitPairTrailing(parts[0], `:`)

	switch len(parts) {
	case 1: // no version specified, assume to be a local import
		if alias != `` {
			return fmt.Sprintf("import %q as %s", lib, alias), nil
		} else if strings.ToLower(filepath.Ext(lib)) == `.js` { // script imports require an alias (qualifier)
			alias = strings.TrimSuffix(filepath.Base(lib), filepath.Ext(lib))

			if unicode.IsLower(rune(alias[0])) {
				alias = stringutil.Camelize(alias)
			}

			return fmt.Sprintf("import %q as %s", lib, alias), nil
		} else {
			return fmt.Sprintf("import %q", lib), nil
		}
	default: // version specified, import from QML_IMPORT_PATH
		version := parts[1]

		if alias != `` {
			return fmt.Sprintf("import %s %s as %s", lib, version, alias), nil
		} else {
			return fmt.Sprintf("import %s %s", lib, version), nil
		}
	}
}
