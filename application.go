package qmlgen

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghodss/yaml"
)

type Application struct {
	Imports      []string     `json:"imports,omitempty"`
	Dependencies []Dependency `json:"dependencies,omitempty"`
	Modules      []*Module    `json:"modules,omitempty"`
	Root         *Component   `json:"root,omitempty"`
	ModuleRoot   string       `json:"module_root"`
	filename     string
}

func LoadFile(yamlFilename string) (*Application, error) {
	if fn, err := fileutil.ExpandUser(yamlFilename); err == nil {
		if file, err := os.Open(fn); err == nil {
			defer file.Close()

			if data, err := ioutil.ReadAll(file); err == nil {
				var app Application

				if err := yaml.Unmarshal(data, &app); err == nil {
					app.filename = yamlFilename

					return &app, nil
				} else {
					return nil, fmt.Errorf("parse: %v", err)
				}
			} else {
				return nil, fmt.Errorf("read: %v", err)
			}
		} else {
			return nil, fmt.Errorf("open: %v", err)
		}
	} else {
		return nil, fmt.Errorf("path: %v", err)
	}
}

func (self *Application) WriteDependencies() error {
	for i, dep := range self.Dependencies {
		if dep.Name == `` {
			return fmt.Errorf("dependency %d: must provide a name", i)
		}

		if tgt := filepath.Join(self.ModuleRoot, dep.Name); fileutil.IsNonemptyFile(tgt) {
			log.Debugf("dependency %q: %s", dep.Name, tgt)
			continue
		} else if rc, err := dep.Retrieve(); err == nil {
			defer rc.Close()

			if out, err := os.Create(tgt); err == nil {
				defer out.Close()

				if n, err := io.Copy(out, rc); err == nil {
					log.Debugf("wrote dependency %q (%d bytes)", dep.Name, n)
					out.Close()
				} else {
					return fmt.Errorf("write dependency %q: %v", dep.Name, err)
				}
			} else {
				return fmt.Errorf("dependency %q: %v", dep.Name, err)
			}

			rc.Close()
		} else {
			return fmt.Errorf("dependency %q: %v", dep.Name, err)
		}
	}

	return nil
}

func (self *Application) WriteModules() error {
	for _, mod := range self.Modules {
		if out, err := os.Create(filepath.Join(self.ModuleRoot, mod.Name+`.qml`)); err == nil {
			defer out.Close()

			for i, imp := range mod.Imports {
				if stmt, _, err := self.toImportStatement(i, imp); err == nil {
					mod.Imports[i] = stmt
					out.WriteString(stmt + "\n")
				} else {
					return fmt.Errorf("module %q: import %s: %s", mod.Name, imp, err)
				}
			}

			if defn := mod.Definition; defn != nil {
				if data, err := defn.QML(0); err == nil {
					if _, err := out.Write(data); err != nil {
						return fmt.Errorf("module %q: write error %v", mod.Name, err)
					}

					out.Close()
				} else {
					return err
				}
			} else {
				return fmt.Errorf("module %q: must provide a definition", mod.Name)
			}
		} else {
			return fmt.Errorf("write module %v: %s", mod.Name, err)
		}
	}

	return nil
}

func (self *Application) QML() ([]byte, error) {
	var out bytes.Buffer
	var writeback bool

	for i, imp := range self.Imports {
		if stmt, wb, err := self.toImportStatement(i, imp); err == nil {
			out.WriteString(stmt + "\n")

			if wb {
				writeback = true
			}
		} else {
			return nil, err
		}
	}

	if err := self.WriteDependencies(); err != nil {
		return nil, err
	}

	if err := self.WriteModules(); err == nil {
		out.WriteString(fmt.Sprintf("import %q\n", `.`))
	} else {
		return nil, err
	}

	out.WriteString("\n")

	if writeback && self.filename != `` {
		if data, err := yaml.Marshal(self); err == nil {
			if _, err := fileutil.WriteFile(data, self.filename); err != nil {
				return nil, fmt.Errorf("failed to update YAML: %v", err)
			}
		} else {
			return nil, fmt.Errorf("failed to update YAML: %v", err)
		}
	}

	if root := self.Root; root != nil {
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

func (self *Application) String() string {
	if data, err := self.QML(); err == nil {
		return string(data)
	} else {
		return ``
	}
}

func (self *Application) toImportStatement(i int, imp string) (string, bool, error) {
	parts := rxutil.Whitespace.Split(imp, -1)

	switch len(parts) {
	case 1:
		return `import ` + parts[0], false, nil
	default:
		lib := parts[0]
		ver := parts[1]

		if major, directive := stringutil.SplitPair(ver, `@`); directive != `` {
			log.Debugf("qmllib: detect %q %q", lib, major+`.x`)

			// identify the latest minor version of a library installed
			if ver, err := resolveVersion(lib, major); err == nil {
				self.Imports[i] = lib + ` ` + ver

				log.Debugf("qmllib: use %q %q", lib, ver)
				return `import ` + self.Imports[i], true, nil
			} else {
				return ``, false, fmt.Errorf("import %v: %v", lib, err)
			}
		} else {
			log.Debugf("qmllib: use %q %q", lib, ver)
			return `import ` + lib + ` ` + ver, false, nil
		}
	}
}
