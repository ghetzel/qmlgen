package hydra

import (
	"fmt"
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

func IsValidModuleFile(path string) bool {
	if file, err := os.Open(path); err == nil {
		defer file.Close()

		if data, err := ioutil.ReadAll(file); err == nil {
			var mod Module

			if err := yaml.UnmarshalStrict(data, &mod); err == nil {
				return true
			}
		}
	}

	return false
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

func LoadModule(uri string, module *Module) error {
	if _, rc, err := fetch(uri); err == nil {
		defer rc.Close()

		if data, err := ioutil.ReadAll(rc); err == nil {
			if module == nil {
				module = new(Module)
			}

			if err := yaml.UnmarshalStrict(data, module); err == nil {
				if strings.TrimSpace(module.Name) == `` {
					module.Name = strings.TrimSuffix(filepath.Base(uri), filepath.Ext(uri))
				}

				log.Debugf("module loaded from: %s", uri)
				return nil
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

func (self *Module) clear() {
	self.Imports = nil
	self.Definition = nil
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

func (self *Module) writeModuleQml(rootDir string, globalImports []string) error {
	qmlfile := fileutil.SetExt(self.RelativePath(), `.qml`)
	qmlfile = env(filepath.Join(rootDir, qmlfile))
	qmlfile, _ = filepath.Abs(qmlfile)
	parentDir := filepath.Dir(qmlfile)

	// no matter what, make sure the rootDir is always a global import
	absRootDir, _ := filepath.Abs(rootDir)
	globalImports = append(globalImports, absRootDir)

	if err := os.MkdirAll(parentDir, 0755); err == nil {
		if defn := self.Definition; defn != nil {
			log.Debugf("Generating %q", qmlfile)

			if out, err := os.Create(qmlfile); err == nil {
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
				for _, gi := range globalImports {
					if !filepath.IsAbs(gi) {
						gi, _ = filepath.Abs(filepath.Join(rootDir, gi))
					}

					if fileutil.DirExists(gi) {
						if rel, err := filepath.Rel(parentDir, gi); err == nil {
							switch rel {
							case `.`, ``:
								break
							default:
								if stmt, err := toImportStatement(rel); err == nil {
									log.Debugf("    %s", stmt)
									out.WriteString(stmt + "\n")
								} else {
									return fmt.Errorf("module %q: import %s: %s", self.Name, rel, err)
								}
							}
						} else {
							log.Warningf("could not find relative from %q to %q: %v", qmlfile, gi, err)
						}
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
