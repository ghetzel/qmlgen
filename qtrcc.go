package hydra

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// Generates
func ManifestFromDir(dirname string) ([]byte, error) {
	rcc := &RCC{
		Version:   `1.0`,
		Resources: new(QResource),
	}

	if err := filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if info.IsDir() {
			return nil
		} else if strings.HasSuffix(info.Name(), `.qrc`) {
			return nil
		} else {
			rcc.Resources.Files = append(rcc.Resources.Files, QrcFile{
				Name: strings.TrimPrefix(path, dirname+`/`),
			})
			return nil
		}
	}); err == nil {
		if out, err := xml.MarshalIndent(rcc, ``, Indent); err == nil {
			return append([]byte(QrcDoctype), out...), nil
		} else {
			return nil, fmt.Errorf("marshal: %v", err)
		}
	} else {
		return nil, fmt.Errorf("walk dir %s: %v", dirname, err)
	}
}
