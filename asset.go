package hydra

import (
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"
)

type Asset struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Checksum string `json:"checksum,omitempty"`
}

func (self *Asset) Retrieve() (io.ReadCloser, error) {
	if self.Source == `` {
		return nil, fmt.Errorf("Must provide a asset source URI")
	}

	return fetch(env(self.Source))
}

func (self *Asset) RelativePath() string {
	if strings.Contains(self.Source, `://`) {
		if u, err := url.Parse(self.Source); err == nil {
			switch u.Scheme {
			case `file`, ``:
				return filepath.Join(u.Hostname(), u.Path)
			default:
				return strings.TrimPrefix(u.Path, `/`)
			}
		} else {
			panic(fmt.Sprintf("asset: bad url %q: %v", self.Source, err))
		}
	} else {
		return self.Source
	}
}
