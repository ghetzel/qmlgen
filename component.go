package qmlgen

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ghetzel/go-stockutil/typeutil"
)

const Indent = `  `

type Layout struct {
	Fill             string `json:"fill,omitempty"`
	HorizontalCenter string `json:"center"`
	VerticalCenter   string `json:"vcenter"`
}

type Component struct {
	Type       string                 `json:"type,omitempty"`
	ID         string                 `json:"id,omitempty"`
	Public     Properties             `json:"public,omitempty"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Components []*Component           `json:"components,omitempty"`
	Layout     *Layout                `json:"layout,omitempty"`
	private    Properties
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

func (self *Component) Set(key string, value interface{}) {
	if self.Properties == nil {
		self.Properties = make(map[string]interface{})
	}

	self.Properties[key] = value
}

func (self *Component) HasContent() bool {
	if len(self.Public) > 0 {
		return true
	} else if len(self.Properties) > 0 {
		return true
	} else if len(self.Components) > 0 {
		return true
	}

	return false
}

func (self *Component) QML(depth int) ([]byte, error) {
	if err := self.Validate(); err == nil {
		var out bytes.Buffer

		self.applyLayoutProperties()

		if self.ID != `` {
			self.Set(`id`, self.ID)
		}

		if self.HasContent() {
			out.WriteString(self.Type + " {\n")
		} else {
			out.WriteString(self.Type + `{`)
		}

		if err := self.writePublicProperties(&out); err != nil {
			return nil, err
		}

		if err := self.writePrivateProperties(&out); err != nil {
			return nil, err
		}

		// write out children (recursive)
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

func (self *Component) applyLayoutProperties() {
	if layout := self.Layout; layout != nil {
		// handle fill
		if strings.HasPrefix(layout.Fill, `@`) {
			self.Set(`anchors.fill`, layout.Fill)
		} else if typeutil.Bool(layout.Fill) {
			self.Set(`anchors.fill`, `@parent`)
		}

		hc := layout.HorizontalCenter
		vc := layout.VerticalCenter

		if typeutil.Bool(hc) && typeutil.Bool(vc) {
			self.Set(`anchors.centerIn`, `@parent`)
		} else if strings.HasPrefix(hc, `@`) && hc == vc {
			self.Set(`anchors.centerIn`, `@`+hc)
		} else {
			if strings.HasPrefix(hc, `@`) {
				self.Set(`anchors.horizontalCenter`, hc+`.horizontalCenter`)
			} else if typeutil.Bool(hc) {
				self.Set(`anchors.horizontalCenter`, `@parent.horizontalCenter`)
			}

			if strings.HasPrefix(vc, `@`) {
				self.Set(`anchors.verticalCenter`, vc+`.verticalCenter`)
			} else if typeutil.Bool(vc) {
				self.Set(`anchors.verticalCenter`, `@parent.verticalCenter`)
			}
		}
	}
}

func (self *Component) writePublicProperties(buf *bytes.Buffer) error {
	// prep public properties by ensuring they are "exposed"
	for i, _ := range self.Public {
		self.Public[i].expose = true
	}

	// write out public properties
	if properties, err := self.Public.QML(); err == nil {
		for _, line := range lines(properties) {
			buf.WriteString(Indent + line + "\n")
		}

		return nil
	} else {
		return err
	}
}

func (self *Component) writePrivateProperties(buf *bytes.Buffer) error {
	for k, v := range self.Properties {
		self.private = append(self.private, &Property{
			Name:  k,
			Value: v,
		})
	}

	// write out private properties
	if properties, err := self.private.QML(); err == nil {
		for _, line := range lines(properties) {
			buf.WriteString(Indent + line + "\n")
		}

		return nil
	} else {
		return err
	}

}
