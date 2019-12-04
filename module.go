package qmlgen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
)

type Module struct {
	Name       string     `json:"name,omitempty"`
	Source     string     `json:"source,omitempty"`
	Imports    []string   `json:"imports,omitempty"`
	Assets     []Asset    `json:"assets,omitempty"`
	Modules    []*Module  `json:"modules,omitempty"`
	Definition *Component `json:"definition,omitempty"`
}

func (self *Module) clear() {
	self.Imports = nil
	self.Definition = nil
}

func (self *Module) Fetch() error {
	name := self.Name

	if self.Source != `` {
		self.clear()

		if rc, err := fetch(self.Source); err == nil {
			defer rc.Close()

			if data, err := ioutil.ReadAll(rc); err == nil {
				if err := yaml.Unmarshal(data, self); err == nil {
					self.Name = name

					if strings.TrimSpace(self.Name) == `` {
						self.Name = strings.TrimSuffix(filepath.Base(self.Source), filepath.Ext(self.Source))
					}

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
	} else {
		return nil
	}
}

func (self *Module) writeQmlFile(root string) error {
	if err := self.Fetch(); err == nil {
		tgt := filepath.Join(root, self.Name+`.qml`)

		if err := os.MkdirAll(filepath.Dir(tgt), 0755); err == nil {
			if out, err := os.Create(tgt); err == nil {
				defer out.Close()

				for _, imp := range self.Imports {
					if stmt, err := toImportStatement(imp); err == nil {
						out.WriteString(stmt + "\n")
					} else {
						return fmt.Errorf("module %q: import %s: %s", self.Name, imp, err)
					}
				}

				out.WriteString(fmt.Sprintf("import %q\n", `.`))

				if defn := self.Definition; defn != nil {
					if data, err := defn.QML(0); err == nil {
						if _, err := out.Write(data); err != nil {
							return fmt.Errorf("module %q: write error %v", self.Name, err)
						}

						out.Close()
					} else {
						return err
					}
				} else {
					return fmt.Errorf("module %q: must provide a definition", self.Name)
				}
			} else {
				return fmt.Errorf("write module %v: %s", self.Name, err)
			}
		} else {
			return fmt.Errorf("write module %v: %s", self.Name, err)
		}
	} else {
		return fmt.Errorf("fetch module %v: %s", self.Name, err)
	}

	// write out submodules
	for _, mod := range self.Modules {
		if err := mod.writeQmlFile(root); err != nil {
			return err
		}
	}

	// all is well.
	return nil
}
