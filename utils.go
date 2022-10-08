package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"golang.org/x/tools/godoc/vfs"
)

type vfsToHttpFile struct {
	name string
	fs   vfs.FileSystem
	rsc  vfs.ReadSeekCloser
}

func (self *vfsToHttpFile) Read(b []byte) (int, error) {
	return self.rsc.Read(b)
}

func (self *vfsToHttpFile) Seek(offset int64, whence int) (int64, error) {
	return self.rsc.Seek(offset, whence)
}

func (self *vfsToHttpFile) Close() error {
	return self.rsc.Close()
}

func (self *vfsToHttpFile) Readdir(count int) ([]fs.FileInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (self *vfsToHttpFile) Stat() (fs.FileInfo, error) {
	return self.fs.Stat(self.name)
}

type vfsToHttpFsAdapter struct {
	fs vfs.FileSystem
}

func (self *vfsToHttpFsAdapter) Open(name string) (http.File, error) {
	name = filepath.Join(`/src`, name)
	// log.Debugf("vfs: open(%q)", name)
	var f, err = self.fs.Open(name)
	return &vfsToHttpFile{name, self.fs, f}, err
}

type LogLine struct {
	Source    string
	Message   string
	Timestamp time.Time
}
