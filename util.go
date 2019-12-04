package qmlgen

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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

func resolveVersion(lib string, major string) (string, error) {
	for minor := 0; minor < QmlMaxMinorVersion; minor++ {
		if tmpfile, err := fileutil.WriteTempFile(
			fmt.Sprintf("import QtQuick 2.0\nimport %s %s.%d\nItem{}\n", lib, major, minor),
			``,
		); err == nil {
			// log.Debugf("qmllib: try %s %s.%d", lib, major, minor)
			out, err := exec.Command(QmlScene, `--quit`, tmpfile).CombinedOutput()
			os.Remove(tmpfile)

			if err == nil {
				continue
			} else if strings.Contains(string(out), `is not installed`) {
				if minor == 0 {
					break
				} else {
					return fmt.Sprintf("%s.%d", major, minor-1), nil
				}
			} else {
				return ``, fmt.Errorf("qmllib: %v", err)
			}
		} else {
			return ``, fmt.Errorf("tmpfile: %v", err)
		}
	}

	return ``, fmt.Errorf("%s %s.x not found", lib, major)
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
					return fmt.Errorf("http: HTTP %v", res.Status)
				}
			} else {
				return fmt.Errorf("http: %v", err)
			}
		case `file`:
			if f, err := os.Open(fileutil.MustExpandUser(
				filepath.Join(u.Host, u.Path),
			)); err == nil {
				rc = f
			} else {
				return fmt.Errorf("file: %v", err)
			}
		default:
			return fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
	} else {
		return nil, fmt.Errorf("uri: %v", err)
	}

	if rc != nil {
		return rc, nil
	} else {
		return fmt.Errorf("no data")
	}
}
