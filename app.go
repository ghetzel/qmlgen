package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ghetzel/diecast"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/httputil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
	"github.com/mcuadros/go-defaults"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/zipfs"
	"gopkg.in/yaml.v2"
)

var AppSearchPaths = func() []string {
	var head = []string{}
	var tail = []string{
		`.`,
		`~/.cache/hydra/bundles`,
		`/opt/hydra`,
	}

	if hp := os.Getenv(`HYDRA_PATH`); hp != `` {
		for _, p := range stringutil.SplitTrimSpace(hp, `:`) {
			if p == `` {
				continue
			}
			head = append(head, p)
		}
	}

	return append(head, tail...)
}()

var AppConfigFilename = `/app.yaml`
var AppMessageBuffer = 1

type AppFunc func(*App) error

type Message struct {
	ID         string                 `json:"id"`
	Payload    map[string]interface{} `json:"payload"`
	ReceivedAt time.Time              `json:"received_at"`
	SentAt     time.Time              `json:"sent_at"`
}

func (self *Message) Get(key string, fallback ...interface{}) typeutil.Variant {
	return typeutil.V(maputil.Get(self.Payload, key, fallback...))
}

func (self *Message) Set(key string, value interface{}) {
	if self.Payload == nil {
		self.Payload = make(map[string]interface{})
	}

	maputil.Set(self.Payload, key, value)
}

type AppConfig struct {
	URL        string          `yaml:"url,omitempty"      json:"url"`
	Name       string          `yaml:"name"               json:"name"       default:"Hydra App"`
	Width      int             `yaml:"width,omitempty"    json:"height"     default:"800"`
	Height     int             `yaml:"height,omitempty"   json:"width"      default:"600"`
	Fullscreen bool            `yaml:"fullscreen"         json:"fullscreen" default:"false"`
	Backend    *diecast.Server `yaml:"backend,omitempty"  json:"backend"`
	Services   *ProcessManager `yaml:"services,omitempty" json:"services"`
}

type App struct {
	Config   *AppConfig `json:"config"`
	window   Messagable
	path     string
	bundle   []byte
	fs       vfs.FileSystem
	messages chan *Message
}

func (self *App) SetWindow(win Messagable) {
	self.window = win
}

// Ensures that the application configuration is able to be run.
func (self *App) Validate() error {
	if self.fs == nil {
		var r = bytes.NewReader(self.bundle)

		if zr, err := zip.NewReader(r, int64(r.Len())); err == nil {
			self.fs = zipfs.New(&zip.ReadCloser{
				Reader: *zr,
			}, filepath.Base(self.path))
		} else {
			return fmt.Errorf("bad bundle: zip: %v", err)
		}
	}

	// only attempt the config load on the first Validate call (which will make this non-nil)
	if self.Config == nil {
		if appyaml, err := self.fs.Open(AppConfigFilename); err == nil {
			if self.Config == nil {
				self.Config = new(AppConfig)
			}

			if b, err := ioutil.ReadAll(appyaml); err == nil && len(b) > 0 {
				defaults.SetDefaults(self.Config)

				if err := yaml.UnmarshalStrict(b, self.Config); err != nil {
					return fmt.Errorf("app.yaml: %v", err)
				}
			} else {
				return fmt.Errorf("app.yaml: %v", err)
			}
		} else {
			return fmt.Errorf("fs: cannot locate %q: %v", AppConfigFilename, err)
		}
	}

	if self.Config.Backend == nil {
		self.Config.Backend = new(diecast.Server)
	}

	if self.Config.Services == nil {
		self.Config.Services = new(ProcessManager)
	}

	self.messages = make(chan *Message, AppMessageBuffer)

	return nil
}

// Blocking start and run of the application and all containers.
func (self *App) Run(workers ...AppFunc) error {
	if err := self.Validate(); err != nil {
		return err
	}

	// get services going (if any)
	if err := self.Config.Services.Initialize(); err != nil {
		return err
	}

	if self.Config.Backend.Address == `` {
		self.Config.Backend.Address = `127.0.0.1:0`
	}

	// the rootfs is whatever this app bundle's FS is
	self.Config.Backend.SetFileSystem(&vfsToHttpFsAdapter{self.fs})
	self.registerHydraApi(self.Config.Backend)

	// diecast has its *own* async callback mechanism which signals when the server
	// is running on whatever network it's supposed to.  this is especially useful
	// when using the port-zero (:0) notation, as this requests an ephemeral port to
	// listen on, and the callback is the earliest point when the actual port is
	// available for inspection.
	var dcworkers = make([]diecast.ServeFunc, 0)

	for _, worker := range workers {
		dcworkers = append(dcworkers, func(dc *diecast.Server) error {
			self.Config.URL = dc.LocalURL()
			log.Infof("webserver listening at %s", self.Config.URL)
			return worker(self)
		})
	}

	return self.Config.Backend.Serve(dcworkers...)
}

func (self *App) registerHydraApi(dc *diecast.Server) {
	dc.Delete(`/hydra`, func(w http.ResponseWriter, req *http.Request) {
		go self.Config.Services.Stop(false)
		httputil.RespondJSON(w, nil, http.StatusAccepted)
	})

	dc.Get(`/hydra/v1/assets/:path`, func(w http.ResponseWriter, req *http.Request) {
		var path = `/` + dc.P(req, `path`).String()

		if data, err := FS.ReadFile(path); err == nil {
			var cksum = sha512.Sum512(data)

			var contentType = fileutil.GetMimeType(bytes.NewBuffer(data), `application/octet-stream`)

			w.Header().Set(`ETag`, hex.EncodeToString(cksum[:]))
			w.Header().Set(`Content-Type`, contentType)
			w.Header().Set(`Content-Length`, typeutil.String(len(data)))

			w.Write(data)
		} else if os.IsNotExist(err) {
			httputil.RespondJSON(w, err, 404)
		} else {
			httputil.RespondJSON(w, err)
		}
	})

	dc.Get(`/hydra/v1/app`, func(w http.ResponseWriter, req *http.Request) {
		httputil.RespondJSON(w, self)
	})

	dc.Post(`/hydra/v1/message`, func(w http.ResponseWriter, req *http.Request) {
		var msg = new(Message)

		if err := httputil.ParseRequest(req, msg); err == nil {
			if msg.Payload == nil {
				msg.Payload = make(map[string]interface{})
			}

			msg.ReceivedAt = time.Now()

			if reply, err := self.window.Send(msg); err == nil {
				httputil.RespondJSON(w, reply)
			} else {
				httputil.RespondJSON(w, err)
			}
		} else {
			httputil.RespondJSON(w, err)
		}
	})
}

// Load an application from the specified directory or URL pointing to an application bundle, which
// should be a .zip.  If the given path is not a local directory, it is assumed to be a URL.
// Supported schemes for URLs are: http:// https:// ftp:// sftp:// and file://.
func LoadApp(loadpath string) (*App, error) {
	var app = new(App)
	app.path = loadpath

	if fileutil.IsNonemptyDir(loadpath) {
		app.fs = vfs.OS(loadpath)
	} else if bundle, err := fileutil.OpenWithOptions(loadpath, fileutil.OpenOptions{
		Timeout: time.Second,
	}); err == nil {

		if b, err := ioutil.ReadAll(bundle); err == nil {
			app.bundle = b
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}

	if app == nil {
		return nil, fmt.Errorf("failed to load application")
	} else {
		return app, app.Validate()
	}
}

// Attemp to locate an app bundle by searching
func FindAppByName(name string) (*App, error) {
	var candidates = []string{
		name,
	}

	for _, path := range AppSearchPaths {
		candidates = append(candidates, filepath.Join(path, fmt.Sprintf("%s.zip", name)))
	}

	for _, candidate := range candidates {
		if fileutil.Exists(candidate) {
			log.Noticef("find: matched %s", candidate)
			return LoadApp(candidate)
		} else {
			log.Debugf("find: trying %s", candidate)
		}
	}

	return nil, fmt.Errorf("app %q not found", name)
}
