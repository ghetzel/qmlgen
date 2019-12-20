package hydra

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"gopkg.in/yaml.v2"
)

type ModuleSpec struct {
	Global bool `yaml:"global" json:"global"`
}

func LoadModuleSpec(path string) (*ModuleSpec, error) {
	if file, err := os.Open(path); err == nil {
		defer file.Close()

		if data, err := ioutil.ReadAll(file); err == nil {
			spec := new(ModuleSpec)

			if err := yaml.UnmarshalStrict(data, spec); err == nil {
				return spec, nil
			} else {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

type Module struct {
	Name       string     `yaml:"name,omitempty"       json:"name,omitempty"`
	Source     string     `yaml:"source,omitempty"     json:"source,omitempty"`
	Imports    []string   `yaml:"imports,omitempty"    json:"imports,omitempty"`
	Assets     []Asset    `yaml:"assets,omitempty"     json:"assets,omitempty"`
	Modules    []*Module  `yaml:"modules,omitempty"    json:"modules,omitempty"`
	Definition *Component `yaml:"definition,omitempty" json:"definition,omitempty"`
	Singleton  bool       `yaml:"singleton,omitempty"  json:"singleton,omitempty"`
	spec       *ModuleSpec
}

func (self *Module) clear() {
	self.Imports = nil
	self.Definition = nil
}

func (self *Module) fetchAt(srcfile string) error {
	name := self.Name

	if rc, err := fetch(srcfile); err == nil {
		defer rc.Close()

		if data, err := ioutil.ReadAll(rc); err == nil {
			if err := yaml.UnmarshalStrict(data, self); err == nil {
				self.Name = name

				if strings.TrimSpace(self.Name) == `` {
					self.Name = strings.TrimSuffix(filepath.Base(srcfile), filepath.Ext(srcfile))
				}

				specInSameDir := filepath.Join(filepath.Dir(srcfile), ModuleSpecFilename)
				return self.appendFile(specInSameDir, nil)
			} else {
				return fmt.Errorf("parse: %v", err)
			}
		} else {
			return fmt.Errorf("read: %v", err)
		}
	} else {
		return err
	}
}

func (self *Module) Fetch() error {
	self.Source = strings.TrimSpace(self.Source)

	if self.Source != `` {
		self.clear()

		if fileutil.DirExists(self.Source) {
			if strings.TrimSpace(self.Name) == `` {
				self.Name = filepath.Base(self.Source)
			}

			return filepath.Walk(self.Source, func(path string, info os.FileInfo, err error) error {
				if err == nil {
					return self.appendFile(path, info)
				}

				return nil
			})
		} else if strings.ContainsAny(self.Source, `*?[]`) { // looks like a glob, treat it as such
			if entries, err := filepath.Glob(self.Source); err == nil {
				for _, entry := range entries {
					if err := self.appendFile(entry, nil); err != nil {
						return err
					}
				}

				return nil
			}
		}

		return self.fetchAt(self.Source)
	} else {
		return nil
	}
}

func (self *Module) appendFile(path string, info os.FileInfo) error {
	if info == nil {
		if s, err := os.Stat(path); err == nil {
			info = s
		} else if os.IsNotExist(err) {
			return nil
		} else {
			return err
		}
	}

	if !info.IsDir() {
		if info.Size() > 0 {
			if filepath.Base(path) == ModuleSpecFilename {
				// we set this flag here because we may only get one shot at parsing this file (depending on
				// where it comes from), so if we see a specfile, we parse it and set it
				self.spec, _ = LoadModuleSpec(path)
			} else {
				switch ext := strings.ToLower(filepath.Ext(path)); ext {
				case `.yaml`, `.yml`:
					submodule := &Module{
						Source: path,
					}

					if err := submodule.Fetch(); err == nil {
						self.Modules = append(self.Modules, submodule)
					} else {
						return err
					}
				default:
					self.Assets = append(self.Assets, Asset{
						Source: path,
					})
				}
			}
		}
	}

	return nil
}

// This function retrieves external assets for this module and all submodules recursively
// and writes them to disk.
func (self *Module) WriteAssets(outdir string) error {
	for _, asset := range self.Assets {
		asset.Name = asset.RelativePath()

		tgt := filepath.Join(outdir, asset.Name)
		tgt = env(tgt)

		if fileutil.IsNonemptyFile(tgt) {
			log.Debugf("asset %q: %s", asset.Name, tgt)
			continue
		} else if rc, err := asset.Retrieve(); err == nil {
			defer rc.Close()

			// ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(tgt), 0755); err == nil {
				// open the destination file for writing
				if out, err := os.Create(tgt); err == nil {
					defer out.Close()

					// copy data from source to output file
					if n, err := io.Copy(out, rc); err == nil {
						log.Debugf("wrote asset %q (%d bytes)", asset.Name, n)
						out.Close()
					} else {
						return fmt.Errorf("write asset %q: %v", asset.Name, err)
					}
				} else {
					return fmt.Errorf("asset %q: %v", asset.Name, err)
				}

				rc.Close()
			} else {
				return fmt.Errorf("asset %q: %v", asset.Name, err)
			}
		} else {
			return fmt.Errorf("asset %q: %v", asset.Name, err)
		}
	}

	for _, mod := range self.Modules {
		if err := mod.WriteAssets(outdir); err != nil {
			return fmt.Errorf("module %s: %v", err)
		}
	}

	return nil
}

func (self *Module) RelativePath() string {
	if self.Source != `` {
		return relativePathFromSource(self.Source)
	} else {
		return self.Name + `.yaml`
	}
}

func (self *Module) AbsolutePath(outdir string) string {
	abs := filepath.Join(outdir, self.RelativePath())
	abs, _ = filepath.Abs(abs)

	return abs
}

// This function writes inline modules out to files.  Modules can optionally be
// sourced from a remote location, in which case this function will retrieve the
// data from that location first.
func (self *Module) WriteModules(app *Application, outdir string) error {
	if err := self.Fetch(); err == nil {
		qmlfile := fileutil.SetExt(self.RelativePath(), `.qml`)
		tgt := env(filepath.Join(outdir, qmlfile))
		tgt, _ = filepath.Abs(tgt)

		if err := os.MkdirAll(filepath.Dir(tgt), 0755); err == nil {
			if defn := self.Definition; defn != nil {
				log.Debugf("Generating %q", tgt)

				if out, err := os.Create(tgt); err == nil {
					defer out.Close()

					if self.Singleton {
						log.Debugf("  singleton: true")
						out.WriteString("pragma Singleton\n")
					}

					log.Debugf("  imports:")

					for _, imp := range self.Imports {
						if stmt, err := toImportStatement(imp); err == nil {
							log.Debugf("    %s", stmt)
							out.WriteString(stmt + "\n")
						} else {
							return fmt.Errorf("module %q: import %s: %s", self.Name, imp, err)
						}
					}

					// add paths that are supposed to be exposed to every module
					for _, abs := range app.GlobalImportPaths() {
						if rel, err := filepath.Rel(filepath.Dir(tgt), abs); err == nil {
							if stmt, err := toImportStatement(rel); err == nil {
								log.Debugf("    %s", stmt)
								out.WriteString(stmt + "\n")
							} else {
								return fmt.Errorf("module %q: import %s: %s", self.Name, rel, err)
							}
						} else {
							log.Warningf("could not find relative from %q to %q: %v", tgt, abs, err)
						}
					}

					// import the current directory
					log.Debugf("    import %q", `.`)
					out.WriteString(fmt.Sprintf("import %q\n", `.`))

					log.Debugf("  type: %v", defn.Type)
					log.Debugf("  signals:")
					for _, sig := range defn.Signals {
						v, _ := sig.QML()
						log.Debugf("    %s", string(v))
					}

					if len(defn.Public) > 0 {
						log.Debugf("  publics:    %d", len(defn.Public))
					}
					if len(defn.Functions) > 0 {
						log.Debugf("  functions:  %d", len(defn.Functions))
					}
					if len(defn.Properties) > 0 {
						log.Debugf("  properties: %d", len(defn.Properties))
					}
					if len(defn.Components) > 0 {
						log.Debugf("  components: %d", len(defn.Components))
					}

					if data, err := defn.QML(0); err == nil {
						if _, err := out.Write(data); err != nil {
							return fmt.Errorf("module %q: write error %v", self.Name, err)
						}

						out.Close()
					} else {
						return err
					}
				} else {
					return fmt.Errorf("write module %v: %s", self.Name, err)
				}
			}
		} else {
			return fmt.Errorf("write module %v: %s", self.Name, err)
		}
	} else {
		return fmt.Errorf("fetch module %v: %s", self.Name, err)
	}

	// write out submodules
	for _, mod := range self.Modules {
		if err := mod.WriteModules(app, outdir); err != nil {
			return err
		}
	}

	// all is well.
	return nil
}

func (self *Module) deepSubmodules() (modules []*Module) {
	modules = append(modules, self.Modules...)

	for _, mod := range self.Modules {
		modules = append(modules, mod.deepSubmodules()...)
	}

	return
}
