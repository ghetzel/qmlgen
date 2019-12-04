package qmlgen

import (
	"fmt"
	"io"
)

type Dependency struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Checksum string `json:"checksum,omitempty"`
}

func (self *Dependency) Retrieve() (io.ReadCloser, error) {
	if self.Source == `` {
		return nil, fmt.Errorf("Must provide a dependency source URI")
	}

	return fetch(self.Source)
}
