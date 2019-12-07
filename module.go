package qmlgen

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
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

// This function retrieves external assets for this module and all submodules recursively
// and writes them to disk.
func (self *Module) WriteAssets(outdir string) error {
	for _, asset := range self.Assets {
		if asset.Name == `` {
			asset.Name = filepath.Base(asset.Source)
		}

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

// This function writes inline modules out to files.  Modules can optionally be
// sourced from a remote location, in which case this function will retrieve the
// data from that location first.
func (self *Module) WriteModules(outdir string) error {
	if err := self.Fetch(); err == nil {
		tgt := env(filepath.Join(outdir, self.Name+`.qml`))

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
		if err := mod.WriteModules(outdir); err != nil {
			return err
		}
	}

	// all is well.
	return nil
}
