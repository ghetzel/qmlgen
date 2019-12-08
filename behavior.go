package qmlgen

import (
	"bytes"
	"fmt"
)

type Behavior struct {
	For       string     `json:"for"`
	Animation *Component `json:"animation"`
	// Enabled   bool       `json:"enabled,omitempty"`
}

func (self *Behavior) QML() ([]byte, error) {
	if self.For == `` {
		return nil, fmt.Errorf("Must specify a property the behavior applies to")
	} else if self.Animation == nil {
		return nil, fmt.Errorf("Must define an animation for the %q property", self.For)
	}

	if qml, err := self.Animation.QML(0); err == nil {
		var out bytes.Buffer

		out.WriteString("Behavior on " + self.For + " {\n")

		for _, line := range lines(qml) {
			out.WriteString(Indent + line + "\n")
		}

		out.WriteString("}")

		return out.Bytes(), nil
	} else {
		return nil, err
	}
}
