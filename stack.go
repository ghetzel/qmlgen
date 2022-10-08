package main

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

var AppLogBuffer = 1024
var DefaultStartWait = 10 * time.Second
var ProcessExitMaxWait = 10 * time.Second
var ProcessExitCheckInterval = 125 * time.Millisecond
var DefaultContainerMemory = `512m`
var DefaultContainerSharedMemory = `256m`
var ContainerInspectTimeout = 3 * time.Second
var DefaultContainerTargetAddr = `localhost`
var DefaultContainerRuntime = `docker`
var DefaultContainerPortTransport = `tcp`
var DefaultStackStartTimeout = 30 * time.Second

type Stack struct {
	ID                 string             `yaml:"-"          json:"id"`
	Name               string             `yaml:"name"       json:"name"`
	Engine             string             `yaml:"engine"     json:"engine"`
	Scripts            map[string]string  `yaml:"scripts"    json:"scripts"`
	PrestartContainers []*ContainerConfig `yaml:"prestart"   json:"prestart"`
	Containers         []*ContainerConfig `yaml:"containers" json:"containers"`
	StartTimeout       string             `yaml:"timeout"    json:"timeout"`
	containers         map[string]Container
	states             map[string]bool
	addrs              map[string]string
	running            bool
	stopchan           chan error
	validated          bool
}

func (self *Stack) Stop() error {
	self.stopchan = make(chan error)
	self.running = false
	return <-self.stopchan
}

func (self *Stack) Run() error {
	if err := self.Validate(); err != nil {
		return err
	}

	log.Noticef("Running stack %q", self.ID)

	self.states = make(map[string]bool)
	self.addrs = make(map[string]string)
	self.running = true

	var stackStartTimeout = typeutil.OrDuration(self.StartTimeout, DefaultStackStartTimeout)
	var wg sync.WaitGroup
	var errchan = make(chan error)

	for n, c := range self.containers {
		wg.Add(1)
		go func(w *sync.WaitGroup, name string, container Container) {
			defer w.Done()

			if err := container.Start(); err == nil {
				go func() {
					for !container.IsRunning() {
						time.Sleep(time.Second)
					}
				}()

			startWaitSelect:
				select {
				case sig := <-globalSignal:
					if err := container.Stop(); err == nil {
						errchan <- fmt.Errorf("[%v] Container startup interrupted with signal %v", container, sig)
					} else {
						errchan <- err
					}

					return

				default:
					var start = time.Now()

					for time.Since(start) < DefaultStartWait {
						if container.IsRunning() {
							if addr := container.Address(); addr != `` {
								log.Debugf("[%v] container started at %v", container, addr)
								self.addrs[name] = addr
								break startWaitSelect
							}
						}

						time.Sleep(ProcessExitCheckInterval)
					}

					errchan <- fmt.Errorf("[%v] container did not stay running", container)
					return
				}
			} else {
				errchan <- err
			}
		}(&wg, n, c)
	}

	var startchan = make(chan bool)

	go func(w *sync.WaitGroup) {
		w.Wait()
		startchan <- true
	}(&wg)

	select {
	case <-startchan:
		break

	case <-time.After(stackStartTimeout):
		return fmt.Errorf("container engine did not respond")
	}

	// BLOCK, poll for running state while stack is supposed to run
	log.Infof("containers started")

	for self.running {
		for name, container := range self.containers {
			container.Config().Running = container.IsRunning()
			self.states[name] = container.Config().Running
		}

		time.Sleep(time.Second)
	}

	// STOP: stack has been stopped, stop all containers
	var stoperr error

	for name, container := range self.containers {
		self.states[name] = false
		container.Config().Running = false
		stoperr = log.AppendError(stoperr, container.Stop())
	}

	if self.stopchan != nil {
		self.stopchan <- stoperr
	}

	return stoperr
}

func (self *Stack) HasRunningContainers() bool {
	for _, state := range self.states {
		if state {
			return true
		}
	}

	return false
}

func (self *Stack) Container(name string) (Container, bool) {
	if container, ok := self.containers[name]; ok {
		return container, true
	} else {
		return nil, false
	}
}

func (self *Stack) ContainerNames() []string {
	return maputil.StringKeys(self.containers)
}

func (self *Stack) StartContainer(name string) error {
	if container, ok := self.containers[name]; ok {
		return container.Start()
	} else {
		return fmt.Errorf("no such container %q", name)
	}
}

func (self *Stack) StopContainer(name string) error {
	if container, ok := self.containers[name]; ok {
		return container.Stop()
	} else {
		return fmt.Errorf("no such container %q", name)
	}
}

func (self *Stack) RestartContainer(name string) error {
	if container, ok := self.containers[name]; ok {
		container.Stop()
		self.WaitForContainerStop(name)

		return container.Start()
	} else {
		return fmt.Errorf("no such container %q", name)
	}
}

func (self *Stack) WaitForContainerStop(name string) bool {
	if container, ok := self.containers[name]; ok {
		for container.IsRunning() {
			time.Sleep(250 * time.Millisecond)
		}

		return true
	} else {
		return false
	}
}

func (self *Stack) waitForAllToStop() {
	for self.HasRunningContainers() {
		time.Sleep(100 * time.Millisecond)
	}
}

func (self *Stack) Validate() error {
	if self.validated {
		return nil
	} else {
		defer func() { self.validated = true }()
	}

	if self.ID == `` {
		self.ID = stringutil.UUID().Base58()
	}

	if len(self.Containers) > 0 {
		self.containers = make(map[string]Container)

		for i, config := range self.Containers {
			var name = typeutil.OrString(config.Name, fmt.Sprintf("%s-container-%d", self.ID, i))
			var engine = typeutil.OrString(config.Engine, self.Engine, DefaultContainerRuntime)
			var uri *url.URL
			var arg string

			if !strings.Contains(engine, `://`) {
				engine = engine + `://`
			}

			if u, err := url.Parse(engine); err == nil {
				uri = u
				engine, uri.Scheme = stringutil.SplitPair(u.Scheme, `+`)
				uri.Scheme = typeutil.OrString(uri.Scheme, `default`)
				arg = uri.String()
			} else {
				return fmt.Errorf("invalid container engine: %v", err)
			}

			log.Debugf("container engine=%q args=%q", engine, arg)

			switch engine {
			case `shell`:
				return fmt.Errorf("invalid container engine %q: SOON.", engine)
			case `docker`:
				self.containers[name] = NewDockerContainer(arg)
			case `kubernetes`:
				self.containers[name] = NewKubernetesContainer(arg)
			default:
				return fmt.Errorf("invalid container engine %q", engine)
			}

			var container = self.containers[name]

			if cfg := container.Config(); cfg != nil {
				*cfg = *config
			}
		}
	}

	return nil
}

func (self *Stack) script(id string) (string, error) {
	if err := self.Validate(); err != nil {
		return ``, err
	}

	if len(self.Scripts) > 0 {
		if tpl, ok := self.Scripts[id]; ok {
			tpl = strings.TrimSpace(tpl)

			if len(tpl) > 0 {
				return tpl, nil
			}
		}
	}

	return ``, fmt.Errorf("unknown script %q", id)
}

type Container interface {
	Start() error
	Config() *ContainerConfig
	Validate() error
	Address() string
	IsRunning() bool
	Stop() error
	ID() string
	String() string
	Tail() <-chan *LogLine
}

type ContainerConfig struct {
	Engine          string                 `yaml:"engine"     json:"engine"`
	Hostname        string                 `yaml:"-"          json:"-"`
	Namespace       string                 `yaml:"-"          json:"-"`
	Name            string                 `yaml:"name"       json:"name"`
	User            string                 `yaml:"user"       json:"user"`
	Env             []string               `yaml:"-"          json:"-"`
	Cmd             []string               `yaml:"cmd"        json:"cmd"`
	ImageName       string                 `yaml:"image"      json:"image"`
	Memory          string                 `yaml:"memory"     json:"memory"`
	SharedMemory    string                 `yaml:"-"          json:"-"`
	Ports           []string               `yaml:"-"          json:"-"`
	Volumes         []string               `yaml:"-"          json:"-"`
	Labels          map[string]string      `yaml:"-"          json:"-"`
	Privileged      bool                   `yaml:"privileged" json:"privileged"`
	WorkingDir      string                 `yaml:"pwd"        json:"pwd"`
	UserDirPath     string                 `yaml:"-"          json:"-"`
	TargetAddr      string                 `yaml:"-"          json:"address"`
	ConfigEnv       map[string]interface{} `yaml:"env"        json:"env"`
	ConfigPorts     map[string]int         `yaml:"ports"      json:"ports"`
	Running         bool                   `yaml:"-"          json:"running"`
	RestartInterval string                 `yaml:"interval"   json:"interval"`
}

func (self *ContainerConfig) Validate() error {
	if self.Name == `` {
		return fmt.Errorf("container config: must specify a name")
	}

	if self.ImageName == `` {
		return fmt.Errorf("container config: must specify a container image")
	}

	if len(self.Cmd) == 0 {
		return fmt.Errorf("container config: must provide a command to run inside the container")
	}

	return nil
}

func (self *ContainerConfig) SetTargetPort(port int) string {
	if h, _, err := net.SplitHostPort(self.TargetAddr); err == nil {
		self.TargetAddr = net.JoinHostPort(h, typeutil.String(port))
	}

	return self.TargetAddr
}

func (self *ContainerConfig) AddPort(outer int, inner int, proto string) {
	if proto == `` {
		proto = `tcp`
	}

	self.Ports = append(self.Ports, fmt.Sprintf("%d:%d/%s", outer, inner, proto))
}
