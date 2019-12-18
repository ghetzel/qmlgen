package hydra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/netutil"
)

type RunOptions struct {
	QmlsceneBin           string
	QmlsceneArgs          []string
	WaitForNetworkTimeout time.Duration
	WaitForNetworkAddress string
	ServeAddress          string
	ServeRoot             string
	BuildDir              string
	Entrypoint            string
}

func (self *RunOptions) Valid() error {
	if bin := executil.Which(self.QmlsceneBin); bin != `` {
		self.QmlsceneBin = bin
	} else {
		return fmt.Errorf("cannot locate qmlscene binary named %q", self.QmlsceneBin)
	}

	if self.BuildDir == `` {
		return fmt.Errorf("BuildDir cannot be empty")
	}

	if self.Entrypoint == `` {
		return fmt.Errorf("Entrypoint cannot be empty")
	}

	return nil
}

func Generate(entrypoint string, app *Application) error {
	if app == nil {
		return fmt.Errorf("no application provided")
	}

	app.OutputDir = filepath.Dir(entrypoint)

	// remove existing builddir
	if err := os.RemoveAll(app.OutputDir); err != nil {
		return err
	}

	// create empty builddir
	if err := os.MkdirAll(app.OutputDir, 0755); err != nil {
		return err
	}

	// generate top-level QML (also writes out all dependencies)
	if qml, err := app.QML(); err == nil {
		// generate qmldir file
		if err := app.WriteModuleManifest(); err != nil {
			return fmt.Errorf("manifest error: %v", err)
		}

		// write out entrypoint QML
		if file, err := os.Create(entrypoint); err == nil {
			defer file.Close()
			_, err := file.Write(qml)
			return err
		} else {
			return fmt.Errorf("write: %v", err)
		}
	} else {
		return fmt.Errorf("bad app: %v", err)
	}
}

func RunWithOptions(app *Application, options RunOptions) error {
	if err := options.Valid(); err == nil {
		entrypoint := filepath.Join(options.BuildDir, options.Entrypoint)

		if err := Generate(entrypoint, app); err != nil {
			return fmt.Errorf("generate failed: %v", err)
		}

		// wait for network
		if netwait := options.WaitForNetworkTimeout; netwait != 0 {
			log.Debugf("Waiting for network to come up...")

			if addr := options.WaitForNetworkAddress; addr == `` {
				err = netutil.WaitForGatewayPing(netwait)
			} else {
				err = netutil.WaitForPing(addr, netwait)
			}

			if err == nil {
				log.Debugf("Network is online")
			} else {
				return fmt.Errorf("wait for network failed: %v", err)
			}
		}

		var qmlargs []string

		qmlargs = append(qmlargs, options.QmlsceneArgs...)
		qmlargs = append(qmlargs, filepath.Base(entrypoint))
		errchan := make(chan error)

		if srvaddr := options.ServeAddress; srvaddr != `` {
			log.Debugf("starting HTTP server at %s", srvaddr)

			go func() {
				ServeRoot = options.ServeRoot
				errchan <- Serve(srvaddr)
			}()
		}

		go func() {
			runner := cmd(app.OutputDir, options.QmlsceneBin, qmlargs...)
			log.Debugf("run: %s", strings.Join(runner.Args, ` `))
			errchan <- runner.Run()
		}()

		select {
		case err := <-errchan:
			return err
		}
	} else {
		return fmt.Errorf("invalid option: %v", err)
	}
}

func cmd(root string, name string, args ...string) *executil.Cmd {
	c := executil.Command(name, args...)
	c.Dir = root
	c.OnStdout = func(line string, _ bool) {
		if line != `` {
			log.Debugf("[%s] %s", name, line)
		}
	}

	c.OnStderr = func(line string, _ bool) {
		if line != `` {
			log.Infof("[%s] %s", name, line)
		}
	}

	return c
}
