package main

import (
	"os"
	"path/filepath"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/go-stockutil/log"
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
		cli.StringFlag{
			Name:   `output-dir, o`,
			Usage:  `The output directory to write the QML to.`,
			Value:  `build`,
			EnvVar: `HYDRA_OUTPUT_DIR`,
		},
		cli.StringFlag{
			Name:   `entrypoint`,
			Usage:  `The name of the application QML in the output directory`,
			Value:  `app.qml`,
			EnvVar: `HYDRA_ENTRYPOINT`,
		},
		cli.StringFlag{
			Name:   `app-qrc`,
			Usage:  `The name of the Qt Resource input manifest.`,
			Value:  `app.qrc`,
			EnvVar: `HYDRA_OUTPUT_APPQRC`,
		},
		cli.BoolFlag{
			Name:  `run, r`,
			Usage: `Run the generated project.`,
		},
		cli.StringFlag{
			Name:   `qml-runner, Q`,
			Usage:  `Run the generated project.`,
			EnvVar: `HYDRA_QMLSCENE_BIN`,
			Value:  `qmlscene`,
		},
		cli.BoolFlag{
			Name:   `server, s`,
			Usage:  `Run a built-in Diecast web server.`,
			EnvVar: `HYDRA_SERVER`,
		},
		cli.StringFlag{
			Name:   `address, a`,
			Usage:  `The address the built-in server should listen on (if enabled).`,
			Value:  `127.0.0.1:11647`,
			EnvVar: `HYDRA_SERVER_ADDR`,
		},
		cli.StringFlag{
			Name:   `server-root, R`,
			Usage:  `The root directory containing files the server should serve.`,
			Value:  hydra.ServeRoot,
			EnvVar: `HYDRA_SERVER_ROOT`,
		},
		cli.DurationFlag{
			Name:  `wait-for-network-timeout`,
			Usage: `How long to wait for the network before running the QML anyway (0 = infinite)`,
		},
		cli.StringFlag{
			Name:  `wait-for-network-address`,
			Usage: `If given, this address will be tested for connectivity instead of the default gateway.`,
		},
	}

	app.Before = func(c *cli.Context) error {
		log.SetLevelString(c.String(`log-level`))
		return nil
	}

	app.Action = func(c *cli.Context) {
		appcfg := c.Args().First()

		if app, err := hydra.Load(appcfg); err == nil {
			if c.Bool(`run`) {
				log.FatalIf(hydra.RunWithOptions(app, hydra.RunOptions{
					QmlsceneBin:           c.String(`qml-runner`),
					QmlsceneArgs:          argsAfter(c, `--`),
					WaitForNetworkTimeout: c.Duration(`wait-for-network-timeout`),
					WaitForNetworkAddress: c.String(`wait-for-network-address`),
					ServeAddress:          c.String(`address`),
					ServeRoot:             c.String(`server-root`),
					BuildDir:              c.String(`output-dir`),
					Entrypoint:            c.String(`entrypoint`),
				}))
			} else {
				log.FatalIf(hydra.Generate(
					filepath.Join(c.String(`output-dir`), c.String(`entrypoint`)),
					app,
				))
			}
		} else {
			log.Fatal(err)
		}
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
