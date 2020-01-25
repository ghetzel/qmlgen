package hydra

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/netutil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
)

var DockerContainerQt = executil.Env(`HYDRA_DOCKER_XCB`, `ghetzel/hydra`)

type RunContainment int

const (
	NoContainment RunContainment = iota
	DockerLinuxfbContainment
	DockerXcbContainment
)

func RunContainmentFromString(str string) RunContainment {
	switch str {
	case `docker`, `docker-xcb`:
		return DockerXcbContainment
	case `docker-linuxfb`:
		return DockerLinuxfbContainment
	default:
		return NoContainment
	}
}

type RunOptions struct {
	QmlsceneBin           string
	QmlsceneArgs          []string
	WaitForNetworkTimeout time.Duration
	WaitForNetworkAddress string
	ServeAddress          string
	ServeRoot             string
	ContainmentStrategy   RunContainment
}

func (self *RunOptions) Valid() error {
	if bin := executil.Which(self.QmlsceneBin); bin != `` {
		self.QmlsceneBin = bin
	} else {
		return fmt.Errorf("cannot locate qmlscene binary named %q", self.QmlsceneBin)
	}

	return nil
}

func RunWithOptions(fromDir string, options RunOptions) error {
	if err := options.Valid(); err == nil {
		absBuildDir, _ := filepath.Abs(fromDir)
		entrypoint := filepath.Join(fromDir, fileutil.SetExt(DefaultEntrypointFilename, `.qml`))

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
				errchan <- Serve(srvaddr, fromDir)
			}()
		}

		go func() {
			var runner *executil.Cmd

			switch options.ContainmentStrategy {
			case DockerXcbContainment:
				if xdisplay := os.Getenv(`DISPLAY`); xdisplay != `` {
					hydraXauth := `/tmp/.hydra.` + stringutil.UUID().String() + `.xauth`
					hydraXauthHex := hydraXauth + `.hex`

					defer os.Remove(hydraXauth)
					defer os.Remove(hydraXauthHex)

					// extract session info from xauth
					if err := executil.Command(`xauth`, `nextract`, hydraXauthHex, os.Getenv(`DISPLAY`)).Run(); err != nil {
						errchan <- fmt.Errorf("cannot contain using docker/xcb: failed to create xauth: %v", err)
						return
					}

					// tweak the xauth hex extract (TODO: what am I actually doing here?  finding the format of this file and what these fields mean is...tricky)
					if lines, err := fileutil.ReadAllLines(hydraXauthHex); err == nil {
						// modify the first field to read "ffff" (don't actually know what this does yet)
						for i, line := range lines {
							parts := strings.Split(line, ` `)

							if len(parts) > 0 {
								parts[0] = `ffff`
							}

							lines[i] = strings.Join(parts, ` `)
						}

						// write the lines back to the file
						if _, err := fileutil.WriteFile(strings.Join(lines, "\n"), hydraXauthHex); err != nil {
							errchan <- fmt.Errorf("cannot contain using docker/xcb: failed to write xauth extract: %v", err)
							return
						}
					} else {
						errchan <- fmt.Errorf("cannot contain using docker/xcb: failed to read xauth extract: %v", err)
						return
					}

					if err := executil.Command(`xauth`, `-f`, hydraXauth, `nmerge`, hydraXauthHex).Run(); err == nil {
						os.Remove(hydraXauthHex)
					} else {
						errchan <- fmt.Errorf("cannot contain using docker/xcb: failed to update xauth: %v", err)
						return
					}

					os.Chmod(hydraXauth, 0755)

					// driargs := getDriDockerArgs()

					runner = cmd(``,
						`docker`,
						`run`,
						`--rm`,
						`--interactive`,
						`--network`, `host`,
						`--volume`, absBuildDir+`:/app`,
						`--volume`, `/tmp/.X11-unix:/tmp/.X11-unix`,
						`--volume`, hydraXauth+`:/Xauthority`,
						`--volume`, `/dev:/dev`,
						`--env`, `XAUTHORITY=/Xauthority`,
						`--env`, `DISPLAY=`+xdisplay,
						`--env`, `QT_QPA_PLATFORM=xcb`,
						`--env`, `HYDRA_HOST=`+Domain,
						`--env`, `HYDRA_ENV=`+Environment,
						`--env`, `HYDRA_ID=`+ID,
						DockerContainerQt,
						`qmlscene`,
						qmlargs)
				} else {
					errchan <- fmt.Errorf("cannot contain using docker-xcb: no DISPLAY available")
				}
			case DockerLinuxfbContainment:
				errchan <- fmt.Errorf("cannot contain using docker-linuxfb: not yet implemented")
				return
			default:
				runner = cmd(fromDir, options.QmlsceneBin, qmlargs)
			}

			log.Debugf("run[%s]: %s", runner.Dir, strings.Join(runner.Args, ` `))
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

func cmd(root string, name string, args ...interface{}) *executil.Cmd {
	c := executil.Command(name, sliceutil.Stringify(sliceutil.Flatten(args))...)
	c.InheritEnv = true
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

func getDriDockerArgs() (paths []string) {
	if entries, err := filepath.Glob(`/usr/lib/x86_64-linux-gnu/dri/*`); err == nil {
		for _, entry := range entries {
			if linkTo, err := filepath.EvalSymlinks(entry); err == nil {
				paths = append(paths, `--volume`, linkTo+`:`+entry)
			}
		}
	} else {
		return
	}

	if fileutil.DirExists(`/dev/dri`) {
		paths = append(paths, `--device`, `/dev/dri:/dev/dri:rw`)
	}

	return
}
