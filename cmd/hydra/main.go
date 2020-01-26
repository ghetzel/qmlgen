package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/httputil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/hydra"
)

func main() {
	app := cli.NewApp()
	app.Name = `hydra`
	app.Usage = `Generate functional GUI applications from data structures`
	app.Version = hydra.Version

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   `log-level, L`,
			Usage:  `Level of log output verbosity`,
			Value:  `info`,
			EnvVar: `LOGLEVEL`,
		},
	}

	app.Before = func(c *cli.Context) error {
		log.SetLevelString(c.String(`log-level`))
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:      `build`,
			Usage:     `Generate all QML from the Hydra YAML source directory`,
			ArgsUsage: `[URL_OR_ENTRYPOINT_FILENAME]`,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  `srcdir, s`,
					Usage: `The directory containing the application source YAML files and entrypoint file.`,
					Value: hydra.DefaultSourceDir,
				},
				cli.StringFlag{
					Name:  `entrypoint, e`,
					Usage: `The main file used to start the application.`,
					Value: hydra.DefaultEntrypointFilename,
				},
				cli.StringFlag{
					Name:  `destdir, d`,
					Usage: `Where the generated QML and assets should be placed.`,
					Value: hydra.DefaultBuildDir,
				},
			},
			Action: func(c *cli.Context) {
				if app, err := hydra.Load(
					c.Args().First(),
					filepath.Join(c.String(`srcdir`), c.String(`entrypoint`)),
				); err == nil {
					log.Debugf("loaded from: %s", app.SourceLocation)

					log.FatalIf(app.Build(hydra.BuildOptions{
						DestDir: c.String(`destdir`),
					}))
				} else {
					log.Fatal(err)
				}
			},
		}, {
			Name:      `compile`,
			ArgsUsage: `[ENTRYPOINT_IN_BUILDDIR]`,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  `builddir, s`,
					Usage: `The directory containing the already-built application.`,
					Value: hydra.DefaultBuildDir,
				},
				cli.StringFlag{
					Name:  `cachedir, c`,
					Usage: `Where intermediate build files should be placed.`,
					Value: hydra.DefaultCacheDir,
				},
				cli.StringFlag{
					Name:  `distdir, d`,
					Usage: `Where the compiled binaries and assets should be placed.`,
					Value: hydra.DefaultCompileDir,
				},
				cli.StringFlag{
					Name:  `entrypoint, e`,
					Usage: `The main file used to start the application.`,
					Value: hydra.DefaultEntrypointFilename,
				},
			},
			Action: func(c *cli.Context) {
				entrypoint := sliceutil.OrString(
					c.Args().First(),
					filepath.Join(c.String(`builddir`), c.String(`entrypoint`)),
				)

				if app, err := hydra.Load(entrypoint); err == nil {

					log.FatalIf(app.Compile(hydra.CompileOptions{
						DestDir: c.String(`destdir`),
					}))
				} else {
					log.Fatal(err)
				}
			},
		}, {
			Name: `serve`,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  `run, r`,
					Usage: `Execute the built binary after the server has started.`,
				},
				cli.StringFlag{
					Name:  `bin, b`,
					Usage: `The command to execute.`,
					Value: filepath.Join(hydra.DefaultCompileDir, `app`),
				},
				cli.StringFlag{
					Name:   `address, a`,
					Usage:  `The address the built-in server should listen on (if enabled).`,
					Value:  `127.0.0.1:11647`,
					EnvVar: `HYDRA_SERVER_ADDR`,
				},
			},
			Action: func(c *cli.Context) {
				var root = c.Args().First()

				if root == `` {
					root = hydra.ServeRoot
				}

				if c.Bool(`run`) {
					go func() {
						log.FatalIf(
							httputil.WaitForHTTP(`http://`+c.String(`address`)+`/ping`, 10*time.Second),
						)

						cmd := executil.ShellCommand(c.String(`bin`))
						cmd.InheritEnv = true
						cmd.OnStdout = executil.LogOutput(log.INFO, log.WARNING)
						cmd.OnStderr = executil.LogOutput(log.INFO, log.WARNING)

						log.FatalIf(cmd.Run())
					}()
				}

				log.FatalIf(hydra.Serve(c.String(`address`), root))
			},
		},
	}

	app.Run(os.Args)
}

func argsAfter(c *cli.Context, delim string) (out []string) {
	var doing bool

	for _, arg := range c.Args() {
		if arg == delim {
			doing = true
		} else if doing {
			out = append(out, arg)
		}
	}

	return
}
