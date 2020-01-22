package hydra

//go:generate esc -o static.go -pkg hydra -modtime 1500000000 -prefix templates templates

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
)

const QrcDoctype = "<!DOCTYPE RCC>\n"

type QrcFile struct {
	Name string `xml:",chardata"`
}

type QResource struct {
	Files []QrcFile `xml:"file"`
}

type RCC struct {
	XMLName   xml.Name
	Version   string     `xml:"version,attr"`
	Resources *QResource `xml:"qresource,omitempty"`
}

func qrcSkipFile(filename string) bool {
	base := filepath.Base(filename)
	ext := strings.ToLower(filepath.Ext(base))

	if strings.HasPrefix(filename, `.`) {
		return true
	} else if strings.HasPrefix(base, `.`) {
		return true
	}

	switch ext {
	case `.qrc`, `.qmlc`, `.jsc`:
		return true
	case `.yaml`:
		qml := fileutil.SetExt(filename, `.qml`, `.yaml`)
		return fileutil.FileExists(qml)
	default:
		return false
	}
}

// Generates a Qt Resource file from the files in the given directory.
func QrcFromDir(dirname string) (*RCC, error) {
	rcc := &RCC{
		Version:   `1.0`,
		Resources: new(QResource),
	}

	if err := filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if info.IsDir() {
			return nil
		} else if qrcSkipFile(path) {
			return nil
		} else {
			rcc.Resources.Files = append(rcc.Resources.Files, QrcFile{
				Name: strings.TrimPrefix(path, dirname+`/`),
			})

			return nil
		}
	}); err == nil {
		return rcc, nil
	} else {
		return nil, fmt.Errorf("walk dir %s: %v", dirname, err)
	}
}
