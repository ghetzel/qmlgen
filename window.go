package main

import (
	"embed"
	"fmt"
	"time"
	"unsafe"

	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/typeutil"
	webview "github.com/webview/webview_go"
)

//go:embed lib/js/*.js
var FS embed.FS

var WindowEmbeddedLibraryPath = `lib/js/hydra.js`
var WindowDefaultWidth = 1024
var WindowDefaultHeight = 768
var AppDefaultURL = `about:blank`
var NativeWindowFactory NativeWindowable

type Windowable interface {
	Navigate(url string) error
	SetTitle(t string) error
	Move(x int, y int) error
	Resize(w int, height int) error
	Run() error
	Destroy() error
	Hide() error
}

type NativeWindowable interface {
	Pointer() unsafe.Pointer
}

type Messagable interface {
	Send(*Message) (*Message, error)
}

type Window struct {
	app        *App
	view       webview.WebView
	didInit    bool
	lasterr    error
	fullscreen bool
	w          int
	h          int
}

func CreateWindow(app *App) *Window {
	var win = new(Window)

	if nw := NativeWindowFactory; nw != nil {
		win.view = webview.NewWindow(true, nw.Pointer())
	} else {
		win.view = webview.New(true)
	}

	win.app = app
	app.SetWindow(win)

	return win
}

func (self *Window) init() error {
	if self.view == nil {
		return fmt.Errorf("cannot open window: no view")
	}

	if self.app == nil {
		return fmt.Errorf("cannot open window: no app")
	}

	if self.didInit {
		return nil
	} else {
		if jslib, err := FS.ReadFile(WindowEmbeddedLibraryPath); err == nil {
			self.view.Init(string(jslib))
		} else {
			return err
		}

		self.SetTitle(self.app.Config.Name)
		self.Resize(self.app.Config.Width, self.app.Config.Height)

		if self.app.Config.Fullscreen {
			self.Fullscreen(true)
		}

		self.Navigate(typeutil.OrString(self.app.Config.URL, AppDefaultURL))
		self.didInit = true
	}

	return nil
}

func (self *Window) Run() error {
	if err := self.init(); err != nil {
		return err
	}

	go log.FatalIf(self.app.Run(func(a *App) error {
		go a.Config.Services.Run()
		return nil
	}))

	self.Navigate(typeutil.OrString(self.app.Config.URL, AppDefaultURL))
	self.view.Run()
	self.Wait()

	return self.lasterr
}

func (self *Window) Destroy() error {
	self.app.Config.Services.Stop(false)
	self.view.Destroy()
	return nil
}

func (self *Window) Wait() {
	self.app.Config.Services.Wait()
	log.Debugf("window and all apps stopped")
}

func (self *Window) Navigate(url string) error {
	self.view.Navigate(url)
	return nil
}

func (self *Window) SetTitle(title string) error {
	self.view.SetTitle(title)
	return nil
}

func (self *Window) Move(x int, y int) error {
	return fmt.Errorf("Move: Not Implemented")
}

func (self *Window) Resize(w int, h int) error {
	self.w = w
	self.h = h
	self.view.SetSize(w, h, webview.HintNone)
	return nil
}

func (self *Window) Fullscreen(on bool) error {
	self.fullscreen = on

	if self.fullscreen {
		self.view.SetSize(0, 0, webview.HintMax&webview.HintFixed)
	} else {
		self.Resize(self.w, self.h)
	}

	return nil
}

func (self *Window) Send(req *Message) (*Message, error) {
	var reply = new(Message)
	var err error

	reply.ID = req.ID
	reply.ReceivedAt = req.ReceivedAt
	reply.SentAt = time.Now()

	switch req.ID {
	case `log`:
		var lvl = log.GetLevel(req.Get(`level`, `debug`).String())
		log.Log(lvl, req.Get(`message`, `-- MARK --`).String())

	case `resize`:
		var w = req.Get(`w`, WindowDefaultWidth).NInt()
		var h = req.Get(`h`, WindowDefaultHeight).NInt()
		err = self.Resize(w, h)

	case `move`:
		var x = req.Get(`x`).NInt()
		var y = req.Get(`y`).NInt()
		err = self.Move(x, y)

	case `start`, `stop`, `restart`:
		for _, program := range self.app.Config.Services.Manager.Programs() {
			var e error

			switch req.ID {
			case `start`:
				program.Start()
			case `stop`:
				program.Stop()
			case `restart`:
				program.Restart()
			}

			err = log.AppendError(err, e)
		}

	default:
		err = fmt.Errorf("no such action %q", req.ID)
	}

	return reply, err
}
