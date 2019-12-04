package qmlgen

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghodss/yaml"
)

type Application struct {
	Imports    []string   `json:"imports,omitempty"`
	Assets     []Asset    `json:"assets,omitempty"`
	Modules    []*Module  `json:"modules,omitempty"`
	Root       *Component `json:"root,omitempty"`
	ModuleRoot string     `json:"module_root"`
	filename   string
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

// This function retrieves external assets from various data sources and
// writes them out to files.  These assets may be any type of file.
func (self *Application) WriteAssets() error {
	for _, mod := range self.Modules {
		self.Assets = append(self.Assets, mod.Assets...)
	}

	for _, asset := range self.Assets {
		if asset.Name == `` {
			asset.Name = filepath.Base(asset.Source)
		}

		tgt := filepath.Join(self.ModuleRoot, asset.Name)
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

	return nil
}

// This function writes inline modules out to files.  Modules can optionally be
// sourced from a remote location, in which case this function will retrieve the
// data from that location first.
func (self *Application) WriteModules() error {
	for _, mod := range self.Modules {
		if err := mod.writeQmlFile(self.ModuleRoot); err != nil {
			return err
		}
	}

	return nil
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
	if err := self.WriteModules(); err == nil {
		out.WriteString(fmt.Sprintf("import %q\n", `.`))
	} else {
		return nil, err
	}

	// retrieve and write out all assets
	if err := self.WriteAssets(); err != nil {
		return nil, err
	}

	out.WriteString("\n")

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
