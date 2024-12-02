package main

import (
	"github.com/ghetzel/procwatch"
)

var DefaultProcessManager *ProcessManager

func LoadDefaultProcessManager(pm *ProcessManager) error {
	if pm == nil {
		pm = new(ProcessManager)
	}

	if err := pm.Initialize(); err == nil {
		DefaultProcessManager = pm
		return nil
	} else {
		return err
	}
}

type ProcessManager struct {
	*procwatch.Manager
	Programs []*procwatch.Program `json:"programs"`
	didInit  bool
}

func (self *ProcessManager) Initialize() error {
	if self.Manager == nil {
		self.Manager = procwatch.NewManager()
	}

	if self.didInit {
		return nil
	}

	for _, program := range self.Programs {
		if err := self.AddProgram(program); err != nil {
			return err
		}
	}

	if err := self.Manager.Initialize(); err == nil {
		self.didInit = true
		return nil
	} else {
		return err
	}
}
