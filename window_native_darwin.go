//go:build ignore

package main

import (
	"unsafe"

	"github.com/progrium/macdriver/cocoa"
)

func init() {
	NativeWindowFactory = new(NativeWindow)
}

type NativeWindow struct {
	nswindow *cocoa.NSWindow
}

func (self *NativeWindow) Pointer() unsafe.Pointer {
	if self.nswindow == nil {
		var w = cocoa.NSWindow_Init(cocoa.NSScreen_Main().Frame(), cocoa.NSClosableWindowMask, cocoa.NSBackingStoreBuffered, false)
		w.SetBackgroundColor(cocoa.NSColor_Clear())
		w.SetOpaque(false)
		w.SetTitleVisibility(cocoa.NSWindowTitleHidden)
		w.SetTitlebarAppearsTransparent(true)
		w.SetIgnoresMouseEvents(true)
		w.SetLevel(cocoa.NSMainMenuWindowLevel + 2)
		w.MakeKeyAndOrderFront(w)

		self.nswindow = &w
	}

	return self.nswindow
}
