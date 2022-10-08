package main

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/dustin/go-humanize"
	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/log"
)

type ScriptContainer struct {
	*ContainerConfig
	content  string
	endpoint string
	env      map[string]string
	loglines chan *LogLine
	handle   *executil.Cmd
	id       string
	stack    *Stack
}

func NewScriptContainer(url string, stack *Stack) *ScriptContainer {
	return &ScriptContainer{
		ContainerConfig: &ContainerConfig{},
		endpoint:        url,
		content:         ``,
		env:             make(map[string]string),
		loglines:        make(chan *LogLine, AppLogBuffer),
		stack:           stack,
	}
}

func (self *ScriptContainer) ID() string {
	return self.id
}

func (self *ScriptContainer) String() string {
	return self.Name
}

func (self *ScriptContainer) Tail() <-chan *LogLine {
	return self.loglines
}

func (self *ScriptContainer) Config() *ContainerConfig {
	return self.ContainerConfig
}

func (self *ScriptContainer) Start() error {
	if self.handle == nil {
		if u, err := url.Parse(self.endpoint); err == nil {
			self.id = u.Hostname()
			if self.Name == `` {
				self.Name = self.id
			}

			if self.id != `` && self.stack == nil {
				return fmt.Errorf("cannot reference script by name without stack")
			}

			if tpl, err := self.stack.script(self.id); err == nil {
				self.content = tpl
			} else {
				return err
			}
		} else {
			return err
		}
	}

	return self.handle.Start()
}

func (self *ScriptContainer) logtail() {
	if self.IsRunning() {
		if rc, err := self.client.ContainerLogs(
			context.Background(),
			self.id,
			types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Timestamps: true,
				Follow:     true,
			},
		); err == nil {
			defer rc.Close()

			var linescan = bufio.NewScanner(rc)

			for linescan.Scan() {
				if !self.IsRunning() {
					break
				}

				self.loglines <- &LogLine{
					Source:  self.Config().Name,
					Message: linescan.Text(),
				}
			}
		}
	}
}

func (self *ScriptContainer) Address() string {
	if self.IsRunning() {
		return self.TargetAddr
	} else {
		return ``
	}
}

func (self *ScriptContainer) IsRunning() bool {
	if self.client != nil {
		if self.id != `` {
			ctx, cn := context.WithTimeout(context.Background(), time.Second)
			defer cn()

			if s, err := self.client.ContainerStatPath(ctx, self.id, `/`); err == nil {
				return s.Mode.IsDir()
			}
		}
	}

	return false
}

func (self *ScriptContainer) Stop() error {
	if self.loglines != nil {
		close(self.loglines)
	}

	if self.IsRunning() {
		ctx, cn := context.WithTimeout(context.Background(), ProcessExitMaxWait)
		defer cn()

		if err := self.client.ContainerStop(ctx, self.id, &ProcessExitMaxWait); err == nil {
			ctx, cn := context.WithTimeout(context.Background(), ProcessExitMaxWait)
			defer cn()

			if err := self.client.ContainerRemove(ctx, self.id, types.ContainerRemoveOptions{}); err == nil {
				return nil
			} else if log.ErrContains(err, `already in progress`) {
				return nil
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		return nil
	}

}

func (self *ScriptContainer) Validate() error {
	if self.ImageName == `` {
		return fmt.Errorf("container: must specify an image name")
	}

	if self.Name == `` {
		return fmt.Errorf("container: must be given a name")
	}

	if self.Hostname == `` {
		self.Hostname = self.Name
	}

	if self.Memory == `` {
		self.Memory = DefaultContainerMemory
	}

	if self.SharedMemory == `` {
		self.SharedMemory = DefaultContainerSharedMemory
	}

	if v, err := humanize.ParseBytes(self.Memory); err == nil {
		self.memory = int64(v)
	} else {
		return fmt.Errorf("container-memory: %v", err)
	}

	if v, err := humanize.ParseBytes(self.SharedMemory); err == nil {
		self.shmSize = int64(v)
	} else {
		return fmt.Errorf("container-shm-size: %v", err)
	}

	self.validated = true
	return nil
}
