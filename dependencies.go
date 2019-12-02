package qmlgen

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/ghetzel/go-stockutil/fileutil"
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

	if u, err := url.Parse(self.Source); err == nil {
		switch u.Scheme {
		case `http`, `https`:
			if res, err := http.Get(self.Source); err == nil {
				return res.Body, nil
			} else {
				return nil, fmt.Errorf("http: %v", err)
			}
		case `file`:
			return os.Open(fileutil.MustExpandUser(u.Path))
		default:
			return nil, fmt.Errorf("unsupported source scheme %q", u.Scheme)
		}
	} else {
		return nil, fmt.Errorf("uri: %v", err)
	}
}
