package hydra

import (
	"fmt"
	"net"

	"github.com/ghetzel/diecast"
	"github.com/ghetzel/go-stockutil/fileutil"
)

var ServeRoot = `www`
var DiecastConfig = `diecast.yml`

func Serve(address string) error {
	if _, port, err := net.SplitHostPort(address); err == nil {
		server := diecast.NewServer(ServeRoot)
		server.Address = address
		server.BindingPrefix = fmt.Sprintf("http://127.0.0.1:%s", port)
		server.VerifyFile = ``

		if fileutil.IsNonemptyFile(DiecastConfig) {
			if err := server.LoadConfig(DiecastConfig); err != nil {
				return fmt.Errorf("server config: %v", err)
			}
		}

		return server.Serve()
	} else {
		return fmt.Errorf("bad address: %v", err)
	}
}
