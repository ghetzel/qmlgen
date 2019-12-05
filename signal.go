package qmlgen

import (
	"bytes"
	"fmt"
	"strings"
)

type Argument struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (self Argument) String() string {
	return self.Type + ` ` + self.Name
}

type Signal struct {
	Name      string     `json:"name"`
	Arguments []Argument `json:"args"`
}

func (self *Signal) QML() ([]byte, error) {
	var out bytes.Buffer
	var args []string

	out.WriteString(`signal ` + self.Name + `(`)

	for _, arg := range self.Arguments {
		if arg.Name == `` {
			return nil, fmt.Errorf("argument name missing")
		} else if arg.Type == `` {
			return nil, fmt.Errorf("argument type missing")
		}

		args = append(args, arg.String())
	}

	out.WriteString(strings.Join(args, `, `))
	out.WriteString(")")

	return out.Bytes(), nil
}
