package hydra

import (
	"fmt"
	"io"
)

type Asset struct {
	Name   string `json:"name"`
	Source string `json:"source"`
}

func (self *Asset) Retrieve() (io.ReadCloser, error) {
	if self.Source == `` {
		return nil, fmt.Errorf("Must provide a asset source URI")
	}

	return fetch(env(self.Source))
}

func (self *Asset) RelativePath() string {
	return relativePathFromSource(self.Source)
}
