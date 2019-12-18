package hydra

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"gopkg.in/yaml.v2"
)

var Endpoint string = executil.Env(`HYDRA_ENDPOINT`, `app.yaml`)
var Domain = executil.Env(`HYDRA_HOST`, `hydra.local`)
var Environment = executil.Env(`HYDRA_ENV`)
var ID = executil.Env(`HYDRA_ID`)
var Hostname, _ = os.Hostname()
var Client = &http.Client{
	Timeout: 10 * time.Second,
}

type Application struct {
	Module    `yaml:",inline"`
	OutputDir string `yaml:"-" json:"-"`
	filename  string
}

// Loads an application from the given io.Reader data.
func FromReader(reader io.Reader) (*Application, error) {
	if data, err := ioutil.ReadAll(reader); err == nil {
		var app Application

		if err := yaml.UnmarshalStrict(data, &app); err == nil {
			return &app, nil
		} else {
			return nil, fmt.Errorf("parse: %v", err)
		}
	} else {
		return nil, fmt.Errorf("read: %v", err)
	}
}

// Loads an application from the given YAML filename.
func FromFile(yamlFilename string) (*Application, error) {
	if fn, err := fileutil.ExpandUser(yamlFilename); err == nil {
		if file, err := os.Open(fn); err == nil {
			defer file.Close()

			if app, err := FromReader(file); err == nil {
				app.filename = yamlFilename
				return app, nil
			} else {
				return nil, err
			}
		} else {
			return nil, fmt.Errorf("open: %v", err)
		}
	} else {
		return nil, fmt.Errorf("path: %v", err)
	}
}

// Loads an application from the given URL.
func FromURL(url string) (*Application, error) {
	if res, err := Client.Get(url); err == nil {
		defer res.Body.Close()

		if res.StatusCode < 400 {
			return FromReader(res.Body)
		} else {
			return nil, fmt.Errorf("http: %s", res.Status)
		}
	} else {
		return nil, fmt.Errorf("http: %v", err)
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
			candidates = append(candidates, Environment+`.`+Endpoint)
		}

		candidates = append(candidates, Endpoint)

		if hasEnv {
			candidates = append(candidates, `~/.config/hydra/`+Environment+`.`+Endpoint)
		}

		candidates = append(candidates, `~/.config/hydra/`+Endpoint)

		if hasEnv {
			candidates = append(candidates, `/etc/hydra/`+Environment+`.`+Endpoint)
		}

		candidates = append(candidates, `/etc/hydra/`+Endpoint)

		for _, scheme := range []string{
			`https`,
			`http`,
		} {
			candidates = append(candidates, fmt.Sprintf(
				"%s://%s/%s?env=%s&id=%s&host=%s",
				scheme,
				Domain,
				Endpoint,
				Environment,
				ID,
				Hostname,
			))
		}
	} else {
		candidates = locations
	}

	for _, location := range candidates {
		scheme, _ := stringutil.SplitPair(location, `://`)
		log.Debugf("Load: trying %s", location)

		switch scheme {
		case `https`, `http`:
			if app, err := FromURL(location); err == nil {
				return app, nil
			}
		default:
			if app, err := FromFile(location); err == nil {
				return app, nil
			}
		}
	}

	return nil, fmt.Errorf("no application found by any means")
}

func (self *Application) QML() ([]byte, error) {
	var out bytes.Buffer

	// process all top-level import statements
	for _, imp := range self.Imports {
		if stmt, err := toImportStatement(imp); err == nil {
			out.WriteString(stmt + "\n")
		} else {
			return nil, err
		}
	}

	// retrieve and write out all modules
	if err := self.WriteModules(self.OutputDir); err == nil {
		out.WriteString(fmt.Sprintf("import %q\n", `.`))
	} else {
		return nil, err
	}

	// retrieve and write out all assets
	if err := self.WriteAssets(self.OutputDir); err != nil {
		return nil, err
	}

	out.WriteString("\n")

	if root := self.Definition; root != nil {
		root.ID = `root`

		if data, err := root.QML(0); err == nil {
			out.Write(data)

			return out.Bytes(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("invalid root definition")
	}
}

func (self *Application) WriteModuleManifest() error {
	if qmldir, err := os.Create(filepath.Join(self.OutputDir, `qmldir`)); err == nil {
		defer qmldir.Close()

		if qmlfiles, err := filepath.Glob(filepath.Join(self.OutputDir, `*.qml`)); err == nil {
			w := bufio.NewWriter(qmldir)

			if _, err := w.WriteString("module Application\n"); err != nil {
				return err
			}

			sort.Strings(qmlfiles)

			var singletons []string
			var modules []string

			for _, qmlfile := range qmlfiles {
				if lines, err := fileutil.ReadAllLines(qmlfile); err == nil {
					var singleton bool
					var version string = `1.0`
					var path string = strings.TrimPrefix(qmlfile, self.OutputDir+`/`)
					var base string = strings.TrimSuffix(filepath.Base(qmlfile), filepath.Ext(qmlfile))

					if base == `` {
						continue
					}

				LineLoop:
					for _, line := range lines {
						if rxutil.Match(`^\s*pragma\s+Singleton\s*$`, line) != nil {
							singleton = true
							break LineLoop
						}
					}

					if singleton {
						singletons = append(singletons, `singleton `+base+` `+version+` `+path)
					} else {
						modules = append(modules, base+` `+version+` `+path)
					}
				} else {
					return fmt.Errorf("read %s: %v", qmlfile, err)
				}
			}

			if _, err := w.WriteString(strings.Join(singletons, "\n") + "\n\n"); err != nil {
				return fmt.Errorf("bad write: %v", err)
			}

			if _, err := w.WriteString(strings.Join(modules, "\n") + "\n"); err != nil {
				return fmt.Errorf("bad write: %v", err)
			}

			return w.Flush()
		} else {
			return fmt.Errorf("glob: %v", err)
		}
	} else {
		return fmt.Errorf("qmldir: %v", err)
	}
}

func (self *Application) String() string {
	if data, err := self.QML(); err == nil {
		return string(data)
	} else {
		return ``
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
		} else {
			alias = strings.TrimSuffix(filepath.Base(lib), filepath.Ext(lib))
			return fmt.Sprintf("import %q as %s", lib, alias), nil
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
