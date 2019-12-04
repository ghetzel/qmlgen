package qmlgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

type Property struct {
	Type   string      `json:"type,omitempty"`
	Name   string      `json:"name,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	expose bool
}

func (self Property) qmlvalue() string {
	if self.Value == nil {
		return `null`
	} else {
		s := typeutil.String(self.Value)

		// treat multi-line strings as functions
		if strings.Contains(s, "\n") {
			return "function(){\n" + stringutil.PrefixLines(s, Indent) + "\n}"
		} else if stringutil.IsSurroundedBy(s, `{`, `}`) {
			return strings.TrimSpace(stringutil.Unwrap(s, `{`, `}`))
		} else if strings.HasSuffix(s, `vmin`) {
			f := typeutil.Float(strings.TrimSuffix(s, `vmin`)) / 100.0
			return fmt.Sprintf("((root.height < root.width) ? (root.height * %f) : (root.width * %f))", f, f)

		} else if strings.HasSuffix(s, `vmax`) {
			f := typeutil.Float(strings.TrimSuffix(s, `vmax`)) / 100.0
			return fmt.Sprintf("((root.height > root.width) ? (root.height * %f) : (root.width * %f))", f, f)

		} else if strings.HasSuffix(s, `vw`) {
			f := typeutil.Float(strings.TrimSuffix(s, `vw`)) / 100.0
			return fmt.Sprintf("(root.width * %f)", f)

		} else if strings.HasSuffix(s, `vh`) {
			f := typeutil.Float(strings.TrimSuffix(s, `vh`)) / 100.0
			return fmt.Sprintf("(root.height * %f)", f)

		}

		if data, err := json.Marshal(self.Value); err == nil {
			return string(data)
		} else {
			panic("invalid json: " + err.Error())
		}
	}
}

func (self Property) String() (out string) {
	if self.expose {
		out = `property `

		if self.Type == `` {
			self.Type = `var`
		}
	}

	if self.Type != `` {
		out += self.Type + ` `
	}

	out += self.Name

	if self.Value != nil {
		out += `: ` + self.qmlvalue()
	}

	return
}

type Properties []*Property

func (self Properties) QML() ([]byte, error) {
	var out bytes.Buffer

	for _, property := range self {
		out.WriteString(property.String() + "\n")
	}

	return out.Bytes(), nil
}
