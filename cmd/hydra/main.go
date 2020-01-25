package main

import (
	"os"
	"path/filepath"

	"github.com/ghetzel/cli"
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
		cli.StringFlag{
			Name:  `containment-strategy, C`,
			Usage: `Specify a containment method used to actually run the generated code.`,
		},
		cli.StringFlag{
			Name:  `location, l`,
			Usage: `Specify a source location path or URL where data should be retrieved from.`,
		},
		cli.BoolFlag{
			Name:  `autobuild, B`,
			Usage: `Whether to automatically compile the generated QML into a single binary.`,
		},
		cli.BoolFlag{
			Name:  `preserve, P`,
			Usage: `Whether to skip deleting builddir before building.`,
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
		},
		// {
		// 	Name:  `bundle`,
		// 	Usage: `Package a generated and/or compiled application for distribution.`,
		// 	Action: func(c *cli.Context) {
		// 		if manifest, err := hydra.CreateManifest(from); err == nil {
		// 			if c.Bool(`bundle`) {
		// 				bundleFile := filepath.Join(filepath.Dir(c.String(`output`)), `app.tar.gz`)

		// 				// generate bundle archive
		// 				log.FatalIf(manifest.Bundle(bundleFile))

		// 				// replace manifest with a new one containing only the archive we just created
		// 				bundleManifest := hydra.NewManifest(filepath.Dir(bundleFile))
		// 				bundleManifest.Append(bundleFile)
		// 				bundleManifest.Assets[0].ArchiveFileCount = manifest.FileCount
		// 				bundleManifest.Assets[0].UncompressedSize = manifest.TotalSize
		// 				manifest = bundleManifest
		// 			}

		// 			log.FatalIf(manifest.WriteFile(c.String(`output`)))
		// 		} else {
		// 			log.Fatal(err)
		// 		}
		// 	},
		// },
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
