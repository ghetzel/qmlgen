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
	"strings"
	"time"
	"unicode"

	"github.com/ghetzel/go-stockutil/convutil"
	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
	"gopkg.in/yaml.v2"
)

const EntrypointFilename string = `app.yaml`
const ManifestFilename string = `manifest.yaml`
const ModuleSpecFilename string = `module.yaml`

var Domain = executil.Env(`HYDRA_HOST`, `hydra.local`)
var Environment = executil.Env(`HYDRA_ENV`)
var ID = executil.Env(`HYDRA_ID`)
var Hostname, _ = os.Hostname()
var Client = &http.Client{
	Timeout: 10 * time.Second,
}

func init() {
	os.Setenv(`HYDRA_HOST`, Domain)
	os.Setenv(`HYDRA_ENV`, Environment)
	os.Setenv(`HYDRA_ID`, ID)
}

type Application struct {
	Module   `yaml:",inline"`
	Root     string    `yaml:"-" json:"root"`
	Manifest *Manifest `yaml:"manifest,omitempty"`
	filename string
}

func IsLoadErr(err error) bool {
	if err != nil {
		return strings.HasPrefix(err.Error(), `from-`)
	}

	return false
}

// Loads an application from the given io.Reader data.
func FromReader(app *Application, reader io.Reader) error {
	if app == nil {
		return fmt.Errorf("must provide app instance")
	}

	if data, err := ioutil.ReadAll(reader); err == nil {
		if err := yaml.UnmarshalStrict(data, app); err == nil {
			return nil
		} else {
			return fmt.Errorf("parse: %v", err)
		}
	} else {
		return fmt.Errorf("from-read: %v", err)
	}
}

// Loads an application from the given YAML filename.
func FromFile(app *Application, yamlFilename string) error {
	if fn, err := fileutil.ExpandUser(yamlFilename); err == nil {
		if file, err := os.Open(fn); err == nil {
			defer file.Close()

			if err := FromReader(app, file); err == nil {
				app.filename = yamlFilename
				app.Root = filepath.Dir(yamlFilename)
				return nil
			} else {
				return err
			}
		} else {
			return fmt.Errorf("from-open: %v", err)
		}
	} else {
		return fmt.Errorf("from-path: %v", err)
	}
}

// Loads an application from the given URL.
func FromURL(app *Application, manifestOrAppFileUrl string) error {
	if u, err := url.Parse(manifestOrAppFileUrl); err == nil {
		if res, err := Client.Get(u.String()); err == nil {
			defer res.Body.Close()

			if res.StatusCode < 400 {
				if err := FromReader(app, res.Body); err == nil {
					u.Path = filepath.Dir(u.Path)
					app.Root = u.String()
					return nil
				} else {
					return err
				}
			} else {
				return fmt.Errorf("from-http: %s", res.Status)
			}
		} else {
			return fmt.Errorf("from-http: %v", err)
		}
	} else {
		return fmt.Errorf("from-url: %v", err)
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
			candidates = append(candidates, Environment+`.`+EntrypointFilename)
		}

		candidates = append(candidates, EntrypointFilename)

		if hasEnv {
			candidates = append(candidates, `~/.config/hydra/`+Environment+`.`+EntrypointFilename)
		}

		candidates = append(candidates, `~/.config/hydra/`+EntrypointFilename)

		if hasEnv {
			candidates = append(candidates, `/etc/hydra/`+Environment+`.`+EntrypointFilename)
		}

		candidates = append(candidates, `/etc/hydra/`+EntrypointFilename)

		for _, entrypoint := range []string{
			ManifestFilename,
			EntrypointFilename,
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
		app := new(Application)

		var err error
		scheme, _ := stringutil.SplitPair(location, `://`)
		log.Debugf("Load: trying %s", location)

		switch scheme {
		case `https`, `http`:
			err = FromURL(app, location)
		default:
			err = FromFile(app, location)
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

func (self *Application) ensureManifest(rootDir string) error {
	if self.Manifest == nil {
		if fileutil.DirExists(rootDir) {
			if m, err := CreateManifest(rootDir); err == nil {
				self.Manifest = m
			} else {
				return fmt.Errorf("cannot generate manifest: %v", err)
			}
		} else {
			return fmt.Errorf("cannot generate manifest for root %q", rootDir)
		}
	}

	return nil
}

func (self *Application) Generate(intoDir string) error {
	if err := os.MkdirAll(intoDir, 0700); err != nil {
		return err
	}

	if err := self.ensureManifest(intoDir); err == nil {
		log.Infof("Generating application into directory: %s", intoDir)
		log.Debugf("manifest contains %d files in %v", self.Manifest.FileCount, convutil.Bytes(self.Manifest.TotalSize))

		if err := self.Manifest.Fetch(self.Root, intoDir); err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		var out bytes.Buffer

		if modules, err := self.Manifest.LoadModules(intoDir); err == nil {
			// add standard library functions
			modules = append(self.getBuiltinModules(), modules...)

			// write all modules out to files
			for _, submodule := range modules {
				if err := submodule.writeModuleQml(intoDir, self.Manifest.GlobalImports); err != nil {
					return err
				}

				// if we got an entrypoint *from* the manifest, we assume it's contents
				// should supercede our own
				if submodule.RelativePath() == EntrypointFilename {
					self.Module = *submodule
				}
			}
		} else {
			return err
		}

		// process all top-level import statements
		for _, imp := range self.Imports {
			if stmt, err := toImportStatement(imp); err == nil {
				out.WriteString(stmt + "\n")
			} else {
				return err
			}
		}

		out.WriteString(fmt.Sprintf("import %q\n", `.`))
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

			// write child definitions
			if data, err := root.QML(0, root); err == nil {
				out.Write(data)
			} else {
				return err
			}

			// write out the entrypoint file
			if _, err := fileutil.WriteFile(
				&out,
				filepath.Join(intoDir, fileutil.SetExt(EntrypointFilename, `.qml`)),
			); err == nil {
				// recursively generate all qmldirs
				return self.writeQmlManifest(intoDir)
			} else {
				return err
			}
		} else {
			return fmt.Errorf("invalid root definition")
		}
	} else {
		return err
	}
}

func (self *Application) writeQmlManifest(rootDir string) error {
	if err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
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
		return writeQmldir(rootDir, `Application`)
	} else {
		return err
	}
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
