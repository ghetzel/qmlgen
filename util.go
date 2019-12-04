package qmlgen

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
)

var QmlScene = `qmlscene`
var QmlMaxMinorVersion = 64

func lines(data []byte) (out []string) {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == `` {
			continue
		}

		out = append(out, line)
	}

	return
}

func fetch(uri string) (io.ReadCloser, error) {
	var rc io.ReadCloser

	if u, err := url.Parse(uri); err == nil {
		switch u.Scheme {
		case `http`, `https`:
			if res, err := http.Get(u.String()); err == nil {
				if res.StatusCode < 400 {
					rc = res.Body
				} else {
					return nil, fmt.Errorf("http: HTTP %v", res.Status)
				}
			} else {
				return nil, fmt.Errorf("http: %v", err)
			}
		case `file`, ``:
			if f, err := os.Open(fileutil.MustExpandUser(
				filepath.Join(u.Host, u.Path),
			)); err == nil {
				rc = f
			} else {
				return nil, fmt.Errorf("file: %v", err)
			}
		default:
			return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
	} else {
		return nil, fmt.Errorf("uri: %v", err)
	}

	if rc != nil {
		return rc, nil
	} else {
		return nil, fmt.Errorf("no data")
	}
}
