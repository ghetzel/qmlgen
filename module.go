package qmlgen

import (
	"fmt"
	"io/ioutil"

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
