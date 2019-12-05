package qmlgen

import (
	"bytes"
)

type Property struct {
	Type   string      `json:"type,omitempty"`
	Name   string      `json:"name,omitempty"`
	Value  interface{} `json:"value,omitempty"`
	expose bool
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
		out += `: ` + qmlvalue(self.Value)
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
