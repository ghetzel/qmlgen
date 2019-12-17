package hydra

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ghetzel/go-stockutil/typeutil"
)

const Indent = `  `

type Layout struct {
	Fill             interface{} `yaml:"fill,omitempty" json:"fill,omitempty"`
	HorizontalCenter string      `yaml:"center"         json:"center"`
	VerticalCenter   string      `yaml:"vcenter"        json:"vcenter"`
}

type Component struct {
	Type       string                 `yaml:"type,omitempty"       json:"type,omitempty"`
	ID         string                 `yaml:"id,omitempty"         json:"id,omitempty"`
	Public     Properties             `yaml:"public,omitempty"     json:"public,omitempty"`
	Properties map[string]interface{} `yaml:"properties,omitempty" json:"properties,omitempty"`
	Behaviors  []Behavior             `yaml:"behaviors,omitempty"  json:"behaviors,omitempty"`
	Functions  []Function             `yaml:"functions,omitempty"  json:"functions,omitempty"`
	Components []*Component           `yaml:"components,omitempty" json:"components,omitempty"`
	Layout     *Layout                `yaml:"layout,omitempty"     json:"layout,omitempty"`
	Signals    []*Signal              `yaml:"signals,omitempty"    json:"signals,omitempty"`
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

		// write signal declarations
		if err := self.writeSignals(&out); err != nil {
			return nil, err
		}

		// write properties that are exposed to callers
		if err := self.writePublicProperties(&out); err != nil {
			return nil, err
		}

		// write properties that represent internal state
		if err := self.writePrivateProperties(&out); err != nil {
			return nil, err
		}

		// write out local function definitions
		if err := self.writeFunctions(&out); err != nil {
			return nil, err
		}

		// write behaviors
		if err := self.writeBehaviors(&out); err != nil {
			return nil, err
		}

		// write out subcomponents (recursive)
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
		fill := typeutil.String(layout.Fill)

		if strings.HasPrefix(fill, `@`) {
			self.Set(`anchors.fill`, `{`+strings.TrimPrefix(fill, `@`)+`}`)
		} else if typeutil.Bool(fill) {
			self.Set(`anchors.fill`, `{parent}`)
		}

		hc := layout.HorizontalCenter
		vc := layout.VerticalCenter

		if typeutil.Bool(hc) && typeutil.Bool(vc) {
			self.Set(`anchors.centerIn`, `{parent}`)
		} else if strings.HasPrefix(hc, `@`) && hc == vc {
			self.Set(`anchors.centerIn`, `{`+strings.TrimPrefix(hc, `@`)+`}`)
		} else {
			if strings.HasPrefix(hc, `@`) {
				self.Set(`anchors.horizontalCenter`, `{`+strings.TrimPrefix(hc, `@`)+`.horizontalCenter}`)
			} else if typeutil.Bool(hc) {
				self.Set(`anchors.horizontalCenter`, `{parent.horizontalCenter}`)
			}

			if strings.HasPrefix(vc, `@`) {
				self.Set(`anchors.verticalCenter`, `{`+strings.TrimPrefix(vc, `@`)+`.verticalCenter}`)
			} else if typeutil.Bool(vc) {
				self.Set(`anchors.verticalCenter`, `{parent.verticalCenter}`)
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
	if data, err := self.Public.QML(); err == nil {
		self.writeIndented(buf, data)
		return nil
	} else {
		return err
	}
}

func (self *Component) writePrivateProperties(buf *bytes.Buffer) error {
	self.private = nil

	for k, v := range self.Properties {
		self.private = append(self.private, &Property{
			Name:  k,
			Value: v,
		})
	}

	// write out private properties
	if data, err := self.private.QML(); err == nil {
		self.writeIndented(buf, data)
		return nil
	} else {
		return err
	}

}

func (self *Component) writeSignals(buf *bytes.Buffer) error {
	for _, sig := range self.Signals {
		if data, err := sig.QML(); err == nil {
			self.writeIndented(buf, data)
		} else {
			return err
		}
	}

	return nil
}

func (self *Component) writeFunctions(buf *bytes.Buffer) error {
	for _, fn := range self.Functions {
		if data, err := fn.QML(); err == nil {
			self.writeIndented(buf, data)
		} else {
			return err
		}
	}

	return nil
}

func (self *Component) writeBehaviors(buf *bytes.Buffer) error {
	for _, b := range self.Behaviors {
		if data, err := b.QML(); err == nil {
			self.writeIndented(buf, data)
		} else {
			return err
		}
	}

	return nil
}

func (self *Component) writeIndented(buf *bytes.Buffer, data []byte) {
	for _, line := range lines(data) {
		buf.WriteString(Indent + line + "\n")
	}
}
