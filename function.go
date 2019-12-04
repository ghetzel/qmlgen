package qmlgen

import (
	"bytes"
	"fmt"
	"strings"
)

type Function struct {
	Name       string   `json:"name"`
	Arguments  []string `json:"args"`
	Definition string   `json:"definition"`
}

func (self *Function) Validate() error {
	if self.Name == `` {
		return fmt.Errorf("function must specify a name")
	}

	if self.Definition == `` {
		return fmt.Errorf("function must specify a definition")
	}

	return nil
}

func (self *Function) QML() ([]byte, error) {
	var out bytes.Buffer

	if err := self.Validate(); err == nil {
		out.WriteString("function " + self.Name + "(" + strings.Join(self.Arguments, `, `) + ") {\n")
		for _, line := range lines([]byte(self.Definition)) {
			out.WriteString(Indent + line + "\n")
		}

		out.WriteString("}\n")
	} else {
		return nil, err
	}

	return out.Bytes(), nil
}
