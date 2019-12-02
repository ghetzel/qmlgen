package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/qmlgen"
)

func main() {
	app := cli.NewApp()
	app.Name = `qmlgen`
	app.Usage = `Generate functional GUI applications from data structures`
	app.Version = `0.0.1`

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   `log-level, L`,
			Usage:  `Level of log output verbosity`,
			Value:  `debug`,
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
			Value:  `root.qml`,
			EnvVar: `QMLGEN_OUTPUT_APPQML`,
		},
	}

	app.Before = func(c *cli.Context) error {
		log.SetLevelString(c.String(`log-level`))
		return nil
	}

	app.Action = func(c *cli.Context) {
		if app, err := qmlgen.LoadFile(c.String(`config`)); err == nil {
			app.ModuleRoot = c.String(`output-dir`)

			if qml, err := app.QML(); err == nil {
				switch out := c.String(`output-dir`); out {
				case `-`, ``:
					fmt.Println(string(qml))
				default:
					out = fileutil.MustExpandUser(out)

					if err := os.MkdirAll(out, 0755); err != nil {
						log.Fatal(err)
					}

					if file, err := os.Create(filepath.Join(out, c.String(`app-qml`))); err == nil {
						defer file.Close()

						if _, err := file.Write(qml); err != nil {
							log.Fatalf("write output: %v", err)
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
