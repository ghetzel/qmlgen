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
		if filepath.IsAbs(ServeRoot) {
			rootDir = ServeRoot
		} else {
			rootDir = filepath.Join(rootDir, ServeRoot)
		}

		server := diecast.NewServer(rootDir)
		server.Address = address
		server.BindingPrefix = fmt.Sprintf("http://127.0.0.1:%s", port)
		server.VerifyFile = ``

		dcCfg := filepath.Join(rootDir, DiecastConfig)

		if fileutil.IsNonemptyFile(dcCfg) {
			if err := server.LoadConfig(dcCfg); err != nil {
				return fmt.Errorf("server config: %v", err)
			}
		}

		log.Infof("Serving %s at %s", rootDir, address)
		return server.Serve()
	} else {
		return fmt.Errorf("bad address: %v", err)
	}
}
