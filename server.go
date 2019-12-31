package hydra

import (
	"fmt"
	"net"
	"path/filepath"

	"github.com/ghetzel/diecast"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
)

var ServeRoot = `www`
var DiecastConfig = `diecast.yml`

func Serve(address string, rootDir string) error {
	if _, port, err := net.SplitHostPort(address); err == nil {
		serveDir := rootDir
		dcCfg := filepath.Join(rootDir, DiecastConfig)

		if filepath.IsAbs(ServeRoot) {
			serveDir = ServeRoot
		} else {
			serveDir = filepath.Join(rootDir, ServeRoot)
		}

		server := diecast.NewServer(serveDir)
		server.Address = address
		server.BindingPrefix = fmt.Sprintf("http://127.0.0.1:%s", port)
		server.VerifyFile = ``

		log.Debugf("looking for Diecast config at: %s", dcCfg)

		if fileutil.IsNonemptyFile(dcCfg) {
			if err := server.LoadConfig(dcCfg); err == nil {
				log.Infof("Loaded Diecast config from %s", dcCfg)
			} else {
				return fmt.Errorf("server config: %v", err)
			}
		}

		log.Infof("Serving %s at %s", serveDir, address)
		return server.Serve()
	} else {
		return fmt.Errorf("bad address: %v", err)
	}
}
