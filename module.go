package qmlgen

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"
)

type Module struct {
	Name       string     `json:"name,omitempty"`
	Source     string     `json:"source,omitempty"`
	Imports    []string   `json:"imports,omitempty"`
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
