package qmlgen

import (
	"bytes"
	"fmt"
	"strings"
)

const Indent = `  `

type Component struct {
	Type       string       `json:"type,omitempty"`
	ID         string       `json:"id,omitempty"`
	Properties Properties   `json:"properties,omitempty"`
	Components []*Component `json:"components,omitempty"`
	Fill       string       `json:"fill,omitempty"`
}

func NewComponent(ctype string) *Component {
	return &Component{
		Type: ctype,
	}
}

func (self *Component) Validate() error {
	if self.Type == `` {
		return fmt.Errorf("Component must specify a type.")
	}

	return nil
}

func (self *Component) String() string {
	if data, err := self.QML(0); err == nil {
		return string(data)
	} else {
		panic("generate: " + err.Error())
	}
}

func (self *Component) Property(key string) *Property {
	for _, prop := range self.Properties {
		if prop.Name == key {
			return prop
		}
	}

	return nil
}

func (self *Component) Set(key string, value interface{}) {
	if prop := self.Property(key); prop != nil {
		prop.Value = value
	} else {
		self.Properties = append(self.Properties, &Property{
			Name:  key,
			Value: value,
		})
	}
}

func (self *Component) QML(depth int) ([]byte, error) {
	if err := self.Validate(); err == nil {
		var out bytes.Buffer

		switch strings.ToLower(self.Fill) {
		case `true`, `yes`, `on`:
			self.Set(`anchors.fill`, `@parent`)
		case `false`, `no`, `off`, ``:
			break
		default:
			if strings.HasPrefix(self.Fill, `@`) {
				self.Set(`anchors.fill`, self.Fill)
			} else {
				return nil, fmt.Errorf("invalid fill %q", self.Fill)
			}
		}

		if self.ID != `` {
			self.Set(`id`, self.ID)
		}

		if len(self.Properties) > 0 || len(self.Components) > 0 {
			out.WriteString(self.Type + " {\n")
		} else {
			out.WriteString(self.Type + `{`)
		}

		if properties, err := self.Properties.QML(); err == nil {
			for _, line := range lines(properties) {
				out.WriteString(Indent + line + "\n")
			}
		} else {
			return nil, err
		}

		for _, child := range self.Components {
			if data, err := child.QML(depth + 1); err == nil {
				for _, line := range lines(data) {
					out.WriteString(Indent + line + "\n")
				}
			} else {
				return nil, fmt.Errorf("%s: %s: %v", self.Type, child.Type, err)
			}
		}

		out.WriteString("}")

		return out.Bytes(), nil
	} else {
		return nil, err
	}
}
