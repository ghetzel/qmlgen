package hydra

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghetzel/go-stockutil/convutil"
	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
)

type ManifestFile struct {
	Name   string `yaml:"name"`
	Size   int64  `yaml:"size"`
	SHA256 string `yaml:"sha256"`
	MIME   string `yaml:"mime"`
}

func (self *ManifestFile) ProperName() string {
	return strings.TrimSuffix(filepath.Base(self.Name), filepath.Ext(self.Name))
}

func (self *ManifestFile) Validate(root string) error {
	path := filepath.Join(root, self.Name)

	if fileutil.FileExists(path) {
		if cksum, err := fileutil.ChecksumFile(path, `sha256`); err == nil {
			if hex.EncodeToString(cksum) == self.SHA256 {
				return nil
			} else {
				return fmt.Errorf("invalid local file: ")
			}
		} else {
			return fmt.Errorf("malformed checksum")
		}
	} else {
		return fmt.Errorf("no such file")
	}
}

func (self *ManifestFile) Fetch(root string) (io.ReadCloser, error) {
	return fetch(strings.TrimSuffix(root, `/`) + `/` + strings.TrimPrefix(self.Name, `/`))
}

type ManifestFiles []*ManifestFile

func (self ManifestFiles) TotalSize() (s convutil.Bytes) {
	for _, file := range self {
		s += convutil.Bytes(file.Size)
	}

	return
}

type Manifest struct {
	Assets      ManifestFiles `yaml:"assets"`
	Modules     ManifestFiles `yaml:"modules"`
	GeneratedAt time.Time     `yaml:"generated_at,omitempty"`
	TotalSize   int64         `yaml:"size"`
	FileCount   int64         `yaml:"file_count"`
}

func (self *Manifest) LoadModules(fromDir string) (modules []*Module, err error) {
	for _, file := range self.Modules {
		module := new(Module)

		err = LoadModule(filepath.Join(fromDir, file.Name), module)

		if err == nil {
			module.Source = file.Name
			modules = append(modules, module)
		} else {
			return
		}
	}

	return
}

func (self *Manifest) GetEntrypoint() *ManifestFile {
	for _, file := range self.Modules {
		if file.Name == Entrypoint {
			return file
		}
	}

	return nil
}

func (self *Manifest) Clean(destdir string) error {
	for _, module := range self.Modules {
		os.Remove(filepath.Join(destdir, module.Name))
	}

	return nil
}

func (self *Manifest) Fetch(srcroot string, destdir string) error {
	var toFetch ManifestFiles

	for _, file := range append(self.Assets, self.Modules...) {
		if err := file.Validate(destdir); err != nil {
			toFetch = append(toFetch, file)
		}
	}

	if len(toFetch) > 0 {
		log.Infof("fetching %d files (%v) into %s", len(toFetch), toFetch.TotalSize(), destdir)

		for _, file := range toFetch {
			dest := filepath.Join(destdir, file.Name)
			log.Debugf("fetching file: %s[%s]", srcroot, dest)

			if rc, err := file.Fetch(srcroot); err == nil {
				defer rc.Close()

				if _, err := fileutil.WriteFile(rc, dest); err == nil {
					rc.Close()
				} else {
					return fmt.Errorf("%s: write: %v", file.Name, err)
				}
			} else {
				return fmt.Errorf("%s: retrieve: %v", file.Name, err)
			}
		}
	}

	for _, file := range append(self.Assets, self.Modules...) {
		if err := file.Validate(destdir); err != nil {
			os.Remove(filepath.Join(destdir, file.Name))
			return fmt.Errorf("%s: invalid file: %v", filepath.Join(destdir, file.Name), err)
		}
	}

	return nil
}

// Wherever an app is being developed will have it live in a source tree.  This function will
// walk that tree and generate a manifest.yaml from it.
//
// Local paths will go into the manifest as relative to the approot.
// Remote paths will go into the manifest verbatim, UNLESS a flag is set to download them
// now (VENDORING), in which case they will be local paths.
// ----------------------------------------------------------------------------------------------
func CreateManifest(srcdir string) (*Manifest, error) {
	manifest := new(Manifest)

	if err := filepath.Walk(srcdir, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			if !info.IsDir() {
				if cksum, err := fileutil.ChecksumFile(path, `sha256`); err == nil {
					if rel, err := filepath.Rel(srcdir, path); err == nil {
						if strings.HasPrefix(info.Name(), `.`) || strings.HasPrefix(filepath.Dir(path), `.`) {
							return nil
						}

						entry := &ManifestFile{
							Name:   rel,
							Size:   info.Size(),
							SHA256: hex.EncodeToString(cksum),
							MIME:   fileutil.GetMimeType(path),
						}

						if IsValidModuleFile(path) {
							manifest.Modules = append(manifest.Modules, entry)
							log.Noticef("module: %s (%v)", entry.Name, convutil.Bytes(entry.Size))
						} else {
							manifest.Assets = append(manifest.Assets, entry)
							log.Infof(" asset: %s (%v)", entry.Name, convutil.Bytes(entry.Size))
						}

						manifest.FileCount += 1
						manifest.TotalSize += info.Size()
					} else {
						return fmt.Errorf("%s: %v", path, err)
					}
				} else {
					return fmt.Errorf("%s: %v", path, err)
				}
			}

			return nil
		} else {
			return err
		}
	}); err == nil {
		manifest.GeneratedAt = time.Now()
		return manifest, nil
	} else {
		return nil, err
	}
}
