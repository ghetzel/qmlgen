package hydra

import (
	"bytes"
	"encoding/xml"
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

	"github.com/ghetzel/diecast"
	"github.com/ghetzel/go-defaults"
	"github.com/ghetzel/go-stockutil/convutil"
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

const ManifestFilename string = `manifest.yaml`
const ModuleSpecFilename string = `module.yaml`
const AppQrcFile = `app.qrc`
const ErrorContextLines int = 5

var DefaultQtModules = []string{
	`gui`,
	`network`,
	`qml`,
	`quick`,
	`quickcontrols2`,
	`svg`,
	`widgets`,
}

var DefaultSourceDir = executil.Env(`HYDRA_SRCDIR`, `src`)
var DefaultBuildDir = executil.Env(`HYDRA_BUILDDIR`, `build`)
var DefaultCacheDir = executil.Env(`HYDRA_CACHEDIR`, `cache`)
var DefaultCompileDir = executil.Env(`HYDRA_DISTDIR`, `dist`)
var DefaultEntrypointFilename string = `app.yaml`

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

type BuildOptions struct {
	SourceDir string `yaml:"srcdir"  json:"srcdir"  default:"app"`
	DestDir   string `yaml:"destdir" json:"destdir" default:"build"`
}

type CompileOptions struct {
	PreserveDestDir bool     `yaml:"-"`
	Target          string   `yaml:"target"   json:"target"   default:"app"`
	SourceDir       string   `yaml:"srcdir"   json:"srcdir"   default:"build"`
	DestDir         string   `yaml:"destdir"  json:"destdir"  default:"dist"`
	CacheDir        string   `yaml:"cachedir" json:"cachedir" default:"cache"`
	QT              []string `yaml:"qt"       json:"qt"`
	Plugins         []string `yaml:"plugins"  json:"plugins"`
	Sources         []string `yaml:"sources"  json:"sources"`
	Headers         []string `yaml:"headers"  json:"headers"`
	Trailer         string   `yaml:"trailer"  json:"trailer"`
}

type Application struct {
	Module         `yaml:",inline"`
	SourceLocation string          `yaml:"location,omitempty" json:"location,omitempty"`
	Manifest       *Manifest       `yaml:"manifest,omitempty" json:"manifest,omitempty"`
	BuildOptions   *BuildOptions   `yaml:"build,omitempty"  json:"build,omitempty"`
	CompileOptions *CompileOptions `yaml:"compile,omitempty"  json:"compile,omitempty"`
	filename       string
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
				app.SourceLocation = filepath.Dir(yamlFilename)
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
					app.SourceLocation = u.String()
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
//   ./{HYDRA_ENV}.app.yaml
//   ./app.yaml
//   {HYDRA_OUTPUT_DIR}/{HYDRA_ENV}.app.yaml
//   {HYDRA_OUTPUT_DIR}/app.yaml
//   ~/.config/hydra/{HYDRA_ENV}.app.yaml
//   ~/.config/hydra/app.yaml
//   /etc/hydra/{HYDRA_ENV}.app.yaml
//   /etc/hydra/app.yaml
//   https://{HYDRA_HOST:-hydra.local}/manifest.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//   https://{HYDRA_HOST:-hydra.local}/app.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//   http://{HYDRA_HOST:-hydra.local}/manifest.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//   http://{HYDRA_HOST:-hydra.local}/app.yaml?env={HYDRA_ENV}&id={HYDRA_ID}&host=${HYDRA_HOSTNAME}
//
func Load(locations ...string) (*Application, error) {
	locations = sliceutil.CompactString(locations)

	var candidates []string

	if len(locations) == 0 {
		candidates = append(candidates, Environment+`.`+DefaultEntrypointFilename)
		candidates = append(candidates, DefaultEntrypointFilename)

		candidates = append(candidates, filepath.Join(DefaultBuildDir, Environment+`.`+DefaultEntrypointFilename))
		candidates = append(candidates, filepath.Join(DefaultBuildDir, DefaultEntrypointFilename))

		candidates = append(candidates, `~/.config/hydra/`+Environment+`.`+DefaultEntrypointFilename)
		candidates = append(candidates, `~/.config/hydra/`+DefaultEntrypointFilename)

		candidates = append(candidates, `/etc/hydra/`+Environment+`.`+DefaultEntrypointFilename)
		candidates = append(candidates, `/etc/hydra/`+DefaultEntrypointFilename)

		for _, scheme := range []string{
			`https`,
			`http`,
		} {
			for _, entrypoint := range []string{
				ManifestFilename,
				DefaultEntrypointFilename,
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
		if strings.HasPrefix(filepath.Base(location), `.`) {
			continue
		}

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

// This function guarantees that this application has a valid manifest, generating one if necessary.
func (self *Application) ensureManifest(rootDir string) error {
	if self.Manifest == nil || self.Manifest.FileCount == 0 {
		if fileutil.DirExists(rootDir) {
			manifestFile := filepath.Join(rootDir, ManifestFilename)
			srcDir := ``

			if fileutil.FileExists(manifestFile) {
				if mfile, err := os.Open(manifestFile); err == nil {
					defer mfile.Close()
					self.Manifest = new(Manifest)

					if err := yaml.NewDecoder(mfile).Decode(self.Manifest); err == nil {
						if self.Manifest.TotalSize > 0 {
							return nil
						} else {
							self.Manifest = nil
						}
					} else {
						return fmt.Errorf("invalid manifest %s: %v", manifestFile, err)
					}
				} else {
					return fmt.Errorf("invalid manifest %s: %v", manifestFile, err)
				}
			}

			if fileutil.FileExists(self.SourceLocation) {
				srcDir = filepath.Dir(self.SourceLocation)
			} else if fileutil.DirExists(self.SourceLocation) {
				srcDir = self.SourceLocation
			} else {
				return fmt.Errorf("no manifest found and unable to determine source directory")
			}

			if m, err := CreateManifest(srcDir); err == nil {
				if err := m.refreshGlobalImports(); err == nil {
					self.Manifest = m
				} else {
					return err
				}
			} else {
				return fmt.Errorf("cannot generate manifest: %v", err)
			}
		} else {
			return fmt.Errorf("cannot generate manifest for root %q", rootDir)
		}
	}

	self.Manifest.SetRoot(rootDir)
	return nil
}

func (self *Application) writeQrc(intoDir string) error {
	if qrc, err := self.Manifest.QRC(); err == nil {
		if qrcfile, err := os.Create(filepath.Join(intoDir, AppQrcFile)); err == nil {
			defer qrcfile.Close()

			if out, err := xml.MarshalIndent(qrc, ``, Indent); err == nil {
				if _, err := qrcfile.Write(append([]byte(QrcDoctype), out...)); err == nil {
					return nil
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return err
	}
}

func (self *Application) writeAutogenAssets(options CompileOptions) error {
	fs := FS(false)

	options.QT = sliceutil.UniqueStrings(
		append(options.QT, DefaultQtModules...),
	)

	// if v, err := filepath.Rel(options.DestDir, options.CacheDir); err == nil {
	// 	options.CacheDir = v
	// } else {
	// 	return err
	// }

	// if v, err := filepath.Rel(options.DestDir, options.SourceDir); err == nil {
	// 	options.SourceDir = v
	// } else {
	// 	return err
	// }

	// options.DestDir = `.`

	for file := range maputil.M(_escData).Iter(maputil.IterOptions{
		SortKeys: true,
	}) {
		if file.K == `/` {
			continue
		}

		if src, err := fs.Open(file.K); err == nil {
			defer src.Close()
			var srcrdr io.Reader = src
			dstfile := filepath.Join(options.DestDir, file.K)

			if filepath.Ext(file.K) == `.tmpl` {
				tmpl := diecast.NewTemplate(file.K, diecast.TextEngine)
				tmpl.Funcs(diecast.GetStandardFunctions(nil))

				if err := tmpl.ParseFrom(src); err == nil {
					out := bytes.NewBuffer(nil)

					log.Debugf("autogen: rendering template %s", dstfile)

					if err := tmpl.Render(out, map[string]interface{}{
						`compile`: maputil.M(options).MapNative(`yaml`),
					}, ``); err != nil {
						return fmt.Errorf("render %s: %v", file.K, err)
					}

					dstfile = strings.TrimSuffix(dstfile, `.tmpl`)
					srcrdr = out
				} else {
					return err
				}
			}

			if dst, err := os.Create(dstfile); err == nil {
				defer dst.Close()

				if n, err := io.Copy(dst, srcrdr); err == nil {
					log.Debugf("autogen: wrote %s (%d bytes)", dstfile, n)
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// Retrieve the files from this application's manifest and generate the final QML application.
func (self *Application) Build(options BuildOptions) error {
	defaults.SetDefaults(&options)

	if err := os.RemoveAll(options.DestDir); err != nil {
		return err
	}

	if err := os.MkdirAll(options.DestDir, 0700); err != nil {
		return err
	}

	if err := self.ensureManifest(options.DestDir); err == nil {
		log.Infof("Building application in: %s", options.DestDir)

		if self.Manifest.FileCount > 0 {
			log.Debugf("manifest contains %d files in %v", self.Manifest.FileCount, convutil.Bytes(self.Manifest.TotalSize))
		}

		if err := self.Manifest.Fetch(self.SourceLocation, options.DestDir); err != nil {
			return fmt.Errorf("fetch: %v", err)
		}

		var out bytes.Buffer

		if modules, err := self.Manifest.LoadModules(options.DestDir); err == nil {
			// add standard library functions
			modules = append(self.getBuiltinModules(), modules...)

			// write all modules out to files
			for _, submodule := range modules {
				if err := submodule.writeModuleQml(options.DestDir, self.Manifest.GlobalImports); err != nil {
					return err
				}

				// if we got an entrypoint *from* the manifest, we assume it's contents
				// should supercede our own
				if submodule.RelativePath() == DefaultEntrypointFilename {
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

			onCompleted = `Hydra.root = ` + root.ID + "; Hydra.init()\n" + onCompleted
			root.Properties[`Component.onCompleted`] = Literal(onCompleted)

			// write child definitions
			if data, err := root.QML(0, root); err == nil {
				out.Write(data)
			} else {
				return err
			}

			// write out the manifest we're working with
			if mfile, err := os.Create(filepath.Join(options.DestDir, ManifestFilename)); err == nil {
				defer mfile.Close()

				if err := yaml.NewEncoder(mfile).Encode(&Application{
					Manifest: self.Manifest,
				}); err == nil {
					mfile.Close()
				} else {
					return err
				}
			}

			// write out the entrypoint file
			if _, err := fileutil.WriteFile(
				&out,
				filepath.Join(options.DestDir, fileutil.SetExt(DefaultEntrypointFilename, `.qml`)),
			); err == nil {
				// recursively generate all qmldirs
				if err := self.writeQmlManifest(options.DestDir); err != nil {
					return err
				}

				// generate app.qrc
				if err := self.writeQrc(options.DestDir); err != nil {
					return err
				}

				return nil
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

func (self *Application) Compile(options CompileOptions) error {
	defaults.SetDefaults(&options)

	if err := self.ensureManifest(options.SourceDir); err == nil {
		log.Infof(" Staging resources in: %s", options.CacheDir)
		log.Infof("Compiling application: %s/%s", options.DestDir, options.Target)

		os.RemoveAll(options.DestDir)

		if err := os.MkdirAll(options.CacheDir, 0700); err != nil {
			return err
		}

		if err := os.MkdirAll(options.DestDir, 0700); err != nil {
			return err
		}

		// generate build files
		if err := self.writeAutogenAssets(options); err != nil {
			return err
		}

		for _, program := range []string{
			`qmake`,
			`make`,
		} {
			cmd := executil.Command(program)
			cmd.Dir = options.DestDir
			cmd.OnStdout = func(line string, _ bool) {
				if line != `` {
					log.Debugf("[%s] %s", program, line)
				}
			}

			cmd.OnStderr = func(line string, _ bool) {
				if strings.HasPrefix(line, `Error compiling qml file: `) {

					if parts := strings.Split(line, `:`); len(parts) > 4 {
						qmlfile := filepath.Join(options.DestDir, strings.TrimSpace(parts[1]))
						lineno := int(typeutil.Int(parts[2]))
						charno := int(typeutil.Int(parts[3]))

						if lineno > 0 {
							log.Errorf("[%s] %s", program, line)
							log.Debugf("[%s]      \u256d%s", program, strings.Repeat("\u2500", 69))

							for l := (lineno - ErrorContextLines); l < (lineno + ErrorContextLines); l++ {
								if ctx := fileutil.ShouldGetNthLine(qmlfile, l); ctx != `` {
									if l == lineno {
										log.Debugf("[%s]  %3d \u2502 ${red}%s${reset}", program, l, ctx)
										log.Debugf("[%s]      \u2502 ${white+b}%s${reset}", program, strings.Repeat(` `, charno-1)+`^`)
									} else {
										log.Debugf("[%s]  %3d \u2502 %s", program, l, ctx)
									}
								}
							}

							log.Debugf("[%s]      \u2570%s", program, strings.Repeat("\u2500", 69))

							return
						}
					}

				} else if line != `` {
					return
				}

				log.Errorf("[%s] %s", program, line)
			}

			log.Debugf("running command: %q", program)

			if err := cmd.Run(); err != nil {
				return err
			}
		}

		return nil
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

	if strings.HasPrefix(imp, `qrc:`) {
		return fmt.Sprintf("import %q", imp), nil
	}

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
