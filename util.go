package qmlgen

import (
	"fmt"
	"os"
	"os/exec"
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
