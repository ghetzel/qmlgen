package qmlgen

type Module struct {
	Name       string     `json:"name,omitempty"`
	Imports    []string   `json:"imports,omitempty"`
	Definition *Component `json:"definition,omitempty"`
}
