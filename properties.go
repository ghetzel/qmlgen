package hydra

import (
	"bytes"
	"fmt"

	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

// specifies a list of properties that, when encountered, should be treated as inline
// component declarations IFF the property value is an object.
var ElementalProperties = []string{
	`model`,
	`delegate`,
	`style`,
}

var ForceInlineKey = `_inline`

type Property struct {
	Type     string      `yaml:"type,omitempty"     json:"type,omitempty"`
	Name     string      `yaml:"name,omitempty"     json:"name,omitempty"`
	Value    interface{} `yaml:"value,omitempty"    json:"value,omitempty"`
	EnvVar   string      `yaml:"env,omitempty"      json:"env,omitempty"`
	ReadOnly bool        `yaml:"readonly,omitempty" json:"readonly,omitempty"`
	expose   bool
}

func (self Property) shouldInline() bool {
	if typeutil.IsMap(self.Value) {
		// if the "_inline" value is present, honor it (true or false)
		if inline := maputil.M(self.Value).Get(ForceInlineKey); !inline.IsNil() {
			return inline.Bool()
		} else if sliceutil.ContainsString(ElementalProperties, self.Name) {
			return true
		}
	}

	return false
}

func (self Property) QML() ([]byte, error) {
	var out bytes.Buffer

	if self.expose {
		if self.ReadOnly {
			out.WriteString(`readonly `)
		}

		out.WriteString(`property `)

		if self.Type == `` {
			self.Type = `var`
		}
	}

	if self.Type != `` {
		out.WriteString(self.Type + ` `)
	}

	out.WriteString(self.Name)

	if self.shouldInline() {
		inline := new(Component)

		if err := maputil.TaggedStructFromMap(self.Value, inline, `json`); err == nil {
			if qml, err := inline.QML(0); err == nil {
				out.WriteString(`: `)
				out.Write(qml)
			} else {
				return nil, fmt.Errorf("bad inline: %v", err)
			}
		} else {
			return nil, fmt.Errorf("bad inline: %v", err)
		}
	} else if self.Value != nil {
		var envOverride interface{}

		if self.EnvVar != `` {
			if v := executil.Env(self.EnvVar); v != `` {
				envOverride = typeutil.Auto(v)
			}
		}

		if envOverride == nil {
			out.WriteString(`: ` + qmlvalue(self.Value))
		} else {
			out.WriteString(`: ` + qmlvalue(envOverride))
		}
	}

	return out.Bytes(), nil
}

type Properties []*Property

func (self Properties) QML() ([]byte, error) {
	var out bytes.Buffer

	for _, property := range self {
		if qml, err := property.QML(); err == nil {
			out.Write(qml)
			out.WriteString("\n")
		} else {
			return nil, fmt.Errorf("property %s: %v", property.Name, err)
		}
	}

	return out.Bytes(), nil
}
