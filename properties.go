package qmlgen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
	utilutil "github.com/ghetzel/go-stockutil/utils"
)

type Property struct {
	// Scope Scope
	Name  string      `json:"name,omitempty"`
	Value interface{} `json:"value,omitempty"`
}

func (self Property) qmlvalue() string {
	if self.Value == nil {
		return `null`
	} else {
		s := typeutil.String(self.Value)

		if stringutil.IsSurroundedBy(s, `{`, `}`) {
			return stringutil.Unwrap(s, `{`, `}`)
		} else if strings.HasSuffix(s, `vw`) {
			return fmt.Sprintf("(root.width * %f)", typeutil.Float(s[0:len(s)-2])/100.0)
		} else if strings.HasSuffix(s, `vh`) {
			return fmt.Sprintf("(root.height * %f)", typeutil.Float(s[0:len(s)-2])/100.0)
		}

		if strings.HasPrefix(s, `@`) {
			return s[1:]
		} else if typeutil.IsNumeric(self.Value) || utilutil.IsBoolean(self.Value) {
			return s
		} else {
			return `"` + s + `"`
		}
	}
}

func (self Property) String() string {
	return self.Name + `: ` + self.qmlvalue()
}

type Properties []*Property

func (self Properties) QML() ([]byte, error) {
	var out bytes.Buffer

	for _, property := range self {
		out.WriteString(property.String() + ";\n")
	}

	return out.Bytes(), nil
}
