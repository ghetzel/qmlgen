package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/executil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/qmlgen"
)

func main() {
	app := cli.NewApp()
	app.Name = `qmlgen`
	app.Usage = `Generate functional GUI applications from data structures`
	app.Version = qmlgen.Version

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   `log-level, L`,
			Usage:  `Level of log output verbosity`,
			Value:  `info`,
			EnvVar: `LOGLEVEL`,
		},
		cli.StringFlag{
			Name:   `config, c`,
			Usage:  `The configuration YAML to load.`,
			Value:  `app.yaml`,
			EnvVar: `QMLGEN_CONFIG`,
		},
		cli.StringFlag{
			Name:   `output-dir, o`,
			Usage:  `The output directory to write the QML to.`,
			Value:  `build`,
			EnvVar: `QMLGEN_OUTPUT_DIR`,
		},
		cli.StringFlag{
			Name:   `app-qml`,
			Usage:  `The name of the application QML in the output directory`,
			Value:  `app.qml`,
			EnvVar: `QMLGEN_OUTPUT_APPQML`,
		},
		cli.StringFlag{
			Name:   `app-qrc`,
			Usage:  `The name of the Qt Resource input manifest.`,
			Value:  `app.qrc`,
			EnvVar: `QMLGEN_OUTPUT_APPQRC`,
		},
		cli.StringFlag{
			Name:   `app-rcc`,
			Usage:  `The name of the Qt Resource output file.`,
			Value:  `app.rcc`,
			EnvVar: `QMLGEN_OUTPUT_APPRCC`,
		},
		cli.BoolFlag{
			Name:  `run, r`,
			Usage: `Run the generated project.`,
		},
		cli.StringFlag{
			Name:   `qml-runner, Q`,
			Usage:  `Run the generated project.`,
			EnvVar: `QMLGEN_QMLSCENE_BIN`,
			Value:  `qmlscene`,
		},
		cli.BoolFlag{
			Name:   `server, s`,
			Usage:  `Run a built-in Diecast web server.`,
			EnvVar: `QMLGEN_SERVER`,
		},
		cli.StringFlag{
			Name:   `address, a`,
			Usage:  `The address the built-in server should listen on (if enabled).`,
			Value:  `127.0.0.1:11647`,
			EnvVar: `QMLGEN_SERVER_ADDR`,
		},
		cli.StringFlag{
			Name:   `server-root, R`,
			Usage:  `The root directory containing files the server should serve.`,
			Value:  qmlgen.ServeRoot,
			EnvVar: `QMLGEN_SERVER_ROOT`,
		},
		cli.StringFlag{
			Name:   `rcc-bin`,
			Usage:  `The name of the "rcc" resource bundler utility.`,
			Value:  `rcc`,
			EnvVar: `QMLGEN_RCC_BIN`,
		},
	}

	app.Before = func(c *cli.Context) error {
		log.SetLevelString(c.String(`log-level`))
		return nil
	}

	app.Action = func(c *cli.Context) {
		if app, err := qmlgen.LoadFile(c.String(`config`)); err == nil {
			app.OutputDir = c.String(`output-dir`)
			qmlfile := filepath.Join(app.OutputDir, c.String(`app-qml`))
			// qrcfile := filepath.Join(app.OutputDir, c.String(`app-qrc`))
			// rccfile := filepath.Join(app.OutputDir, c.String(`app-rcc`))

			os.Remove(app.OutputDir)
			log.FatalIf(os.MkdirAll(app.OutputDir, 0755))

			// generate application QML, which also populates all assets and modules in the build directory
			if qml, err := app.QML(); err == nil {
				log.FatalIf(app.WriteModuleManifest())
				// generate a Qt Resource manifest from the build directory contents
				// if manifest, err := qmlgen.ManifestFromDir(app.OutputDir); err == nil {
				// 	if _, err := fileutil.WriteFile(manifest, qrcfile); err == nil {
				// 		// shell out to rcc to generate the qt resource bundle
				// 		log.FatalIf(cmd(
				// 			app.OutputDir,
				// 			c.String(`rcc-bin`),
				// 			`--output`,
				// 			rccfile,
				// 			`--compress`,
				// 			`9`,
				// 			qrcfile,
				// 		).Run())
				// 	} else {
				// 		log.Fatalf("write manifest: %v", err)
				// 	}
				// } else {
				// 	log.Fatalf("bad manifest: %v", err)
				// }

				switch out := c.String(`output-dir`); out {
				case `-`, ``:
					fmt.Println(string(qml))
				default:
					out = fileutil.MustExpandUser(out)

					if err := os.MkdirAll(out, 0755); err != nil {
						log.Fatal(err)
					}

					if file, err := os.Create(qmlfile); err == nil {
						defer file.Close()

						if _, err := file.Write(qml); err != nil {
							log.Fatalf("write output: %v", err)
						}

						if c.Bool(`run`) {
							var qmlargs []string
							var argproc bool

							for _, arg := range c.Args() {
								if arg == `--` {
									argproc = true
								} else if argproc {
									qmlargs = append(qmlargs, arg)
								}
							}

							qmlargs = append(qmlargs, c.String(`app-qml`))

							if c.Bool(`server`) {
								log.Infof("[server] starting HTTP server at %s", c.String(`address`))

								go func() {
									qmlgen.ServeRoot = c.String(`server-root`)
									log.FatalIf(qmlgen.Serve(c.String(`address`)))
								}()
							}

							runner := cmd(app.OutputDir, c.String(`qml-runner`), qmlargs...)
							log.Debugf("run: %s", strings.Join(runner.Args, ` `))
							log.FatalIf(runner.Run())
						}
					} else {
						log.Fatalf("write output: %v", err)
					}
				}
			} else {
				log.Fatalf("qml: %v", err)
			}
		} else {
			log.Fatal(err)
		}
	}

	app.Run(os.Args)
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
