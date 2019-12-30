package hydra

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
	"github.com/ghetzel/go-stockutil/log"
	"github.com/ghetzel/go-stockutil/maputil"
	"github.com/ghetzel/go-stockutil/rxutil"
	"github.com/ghetzel/go-stockutil/sliceutil"
	"github.com/ghetzel/go-stockutil/stringutil"
	"github.com/ghetzel/go-stockutil/typeutil"
)

var QmlScene = `qmlscene`
var QmlMaxMinorVersion = 64

type Literal string

// So...this one's weird.
//
// It is EXTREMELY convenient to be able to use json.Marshal to emit syntactically-valid
// JavaScript for QML's benefit.  However, it's a bit of a stickler about JSON specifically,
// which means that having inline JavaScript that isn't part of the JSON notation emits
// errors.
//
// So we need to emit these JavaScript literals as valid JSON strings, but have some way
// to go back and find them in the resulting JSON string so that we can post-process the
// the string to turn them back into literal JavaScript.
//
// This Marhsaler wraps the literal sequence in double quotes (making it a JSON string),
// but ALSO uses Unicode curly braces ⦃⦄ not likely to appear in valid JavaScript code.
// This lets us find+delete these sequences later.
//
func (self Literal) MarshalJSON() ([]byte, error) {
	return []byte(`"` + "\u2983" + string(self) + "\u2984" + `"`), nil
}

func jsonPostProcess(in []byte) string {
	out := string(in)
	out = strings.Replace(out, "\"\u2983", "", -1)
	out = strings.Replace(out, "\u2984\"", "", -1)

	for {
		if match := rxutil.Match(`\\[uU](?P<chr>[0-9a-fA-F]{4})`, out); match != nil {
			g := strings.TrimLeft(match.Group(`chr`), `0`)

			if chr, err := hex.DecodeString(g); err == nil {
				out = strings.Replace(out, match.Group(0), string(chr), -1)
			} else {
				panic("invalid hex match: " + err.Error())
			}
		} else {
			break
		}
	}

	return out
}

func lines(data []byte) (out []string) {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == `` {
			continue
		}

		out = append(out, line)
	}

	return
}

func fetch(uri string) (string, io.ReadCloser, error) {
	var rc io.ReadCloser
	var baseFilename string

	if u, err := url.Parse(uri); err == nil {
		switch u.Scheme {
		case `http`, `https`:
			if res, err := http.Get(u.String()); err == nil {
				if res.StatusCode < 400 {
					baseFilename = filepath.Base(u.Path)
					rc = res.Body
				} else {
					return ``, nil, fmt.Errorf("http: HTTP %v", res.Status)
				}
			} else {
				return ``, nil, fmt.Errorf("http: %v", err)
			}
		case `file`, ``:
			filename := fileutil.MustExpandUser(
				filepath.Join(u.Host, u.Path),
			)

			baseFilename = filepath.Base(filename)

			if f, err := os.Open(filename); err == nil {
				rc = f
			} else {
				return ``, nil, fmt.Errorf("file: %v", err)
			}
		default:
			return ``, nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
	} else {
		return ``, nil, fmt.Errorf("uri: %v", err)
	}

	if rc != nil {
		return baseFilename, rc, nil
	} else {
		return ``, nil, fmt.Errorf("no data")
	}
}

func env(in interface{}) string {
	return stringutil.ExpandEnv(typeutil.String(in))
}

func qmlstring(value interface{}) string {
	s := typeutil.String(value)
	s = env(s)

	// Detect the use of custom units and expand them into expressions
	if strings.Contains(s, "\n") {
		// treat multi-line strings as functions
		return "function(){\n" + stringutil.PrefixLines(s, Indent) + "\n}"
	} else if stringutil.IsSurroundedBy(s, `{`, `}`) {
		return strings.TrimSpace(stringutil.Unwrap(s, `{`, `}`))
	} else if strings.HasSuffix(s, `vmin`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vmin`)) / 100.0
		return fmt.Sprintf("((Hydra.root.height < Hydra.root.width) ? (Hydra.root.height * %f) : (Hydra.root.width * %f))", f, f)

	} else if strings.HasSuffix(s, `vmax`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vmax`)) / 100.0
		return fmt.Sprintf("((Hydra.root.height > Hydra.root.width) ? (Hydra.root.height * %f) : (Hydra.root.width * %f))", f, f)

	} else if strings.HasSuffix(s, `vw`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vw`)) / 100.0
		return fmt.Sprintf("(Hydra.root.width * %f)", f)

	} else if strings.HasSuffix(s, `vh`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vh`)) / 100.0
		return fmt.Sprintf("(Hydra.root.height * %f)", f)

	} else if strings.HasSuffix(s, `pw`) {
		f := typeutil.Float(strings.TrimSuffix(s, `pw`)) / 100.0
		return fmt.Sprintf("(parent.width * %f)", f)

	} else if strings.HasSuffix(s, `ph`) {
		f := typeutil.Float(strings.TrimSuffix(s, `ph`)) / 100.0
		return fmt.Sprintf("(parent.height * %f)", f)

	} else {
		return ``
	}
}

func qmlvalue(value interface{}) string {
	if value == nil {
		return `null`
	} else {
		if qv := qmlstring(value); qv != `` {
			return qv
		}

		// Environment variable expansion (works on strings, and recursively through objects and arrays)
		if typeutil.IsMap(value) {
			value = maputil.Apply(value, qmlMapValueFunc)
		} else if typeutil.IsArray(value) {
			value = sliceutil.Map(value, qmlSliceValueFunc)
		} else if vS, ok := value.(string); ok {
			value = env(vS)
		}

		// JSONify and return
		if data, err := json.MarshalIndent(value, ``, Indent); err == nil {
			return jsonPostProcess(data)
		} else {
			log.Dump(value)
			panic(fmt.Sprintf("qmlvalue(%T): %v", value, err))
		}
	}
}

func qmlSliceValueFunc(i int, value interface{}) interface{} {
	if qv := qmlstring(value); qv != `` {
		return Literal(qv)
	} else if vS, ok := value.(string); ok {
		return env(vS)
	} else if typeutil.IsMap(value) {
		return maputil.Apply(value, qmlMapValueFunc)
	} else {
		return value
	}
}

func qmlMapValueFunc(key []string, value interface{}) (interface{}, bool) {
	if qv := qmlstring(value); qv != `` {
		return Literal(qv), true
	} else if vS, ok := value.(string); ok {
		return env(vS), true
	} else if typeutil.IsMap(value) {
		return typeutil.MapNative(value), true
	} else {
		return nil, false
	}
}

func writeQmldir(outdir string, modname string) error {
	path := filepath.Join(outdir, `qmldir`)

	if modname == `` {
		modname = stringutil.Camelize(filepath.Base(outdir))
	}

	if qmlfiles, err := filepath.Glob(filepath.Join(outdir, `*.qml`)); err == nil {
		if len(qmlfiles) == 0 {
			return nil
		}

		if qmldir, err := os.Create(path); err == nil {
			// log.Debugf("qmldir: %s", path)
			defer qmldir.Close()

			w := bufio.NewWriter(qmldir)

			if _, err := w.WriteString("module " + modname + "\n"); err != nil {
				return err
			}

			sort.Strings(qmlfiles)

			var singletons []string
			var modules []string

			for _, qmlfile := range qmlfiles {
				if lines, err := fileutil.ReadAllLines(qmlfile); err == nil {
					var singleton bool
					var version string = `1.0`
					var path string = strings.TrimPrefix(qmlfile, outdir+`/`)
					var base string = strings.TrimSuffix(filepath.Base(qmlfile), filepath.Ext(qmlfile))

					if base == `` {
						continue
					}

				LineLoop:
					for _, line := range lines {
						if rxutil.Match(`^\s*pragma\s+Singleton\s*$`, line) != nil {
							singleton = true
							break LineLoop
						}
					}

					if singleton {
						singletons = append(singletons, `singleton `+base+` `+version+` `+path)
					} else {
						modules = append(modules, base+` `+version+` `+path)
					}
				} else {
					return fmt.Errorf("read %s: %v", qmlfile, err)
				}
			}

			if len(singletons) > 0 {
				if _, err := w.WriteString(strings.Join(singletons, "\n") + "\n\n"); err != nil {
					return fmt.Errorf("bad write: %v", err)
				}
			}

			if len(modules) > 0 {
				if _, err := w.WriteString(strings.Join(modules, "\n") + "\n"); err != nil {
					return fmt.Errorf("bad write: %v", err)
				}
			}

			return w.Flush()
		} else {
			return fmt.Errorf("qmldir: %v", err)
		}
	} else {
		return fmt.Errorf("glob: %v", err)
	}
}

func relativePathFromSource(source string) string {
	if strings.Contains(source, `://`) {
		if u, err := url.Parse(source); err == nil {
			switch u.Scheme {
			case `file`, ``:
				return filepath.Join(u.Hostname(), u.Path)
			default:
				return strings.TrimPrefix(u.Path, `/`)
			}
		} else {
			panic(fmt.Sprintf("asset: bad url %q: %v", source, err))
		}
	} else {
		return source
	}
}

type archiveType int

const (
	NoArchive archiveType = iota
	TarGz
)

func getArchiveType(archive string) archiveType {
	archive = strings.ToLower(archive)

	if strings.HasSuffix(archive, `.tar.gz`) {
		return TarGz
	} else {
		return NoArchive
	}
}

func extract(manifest *Manifest, archive string, destdir string) error {
	atype := getArchiveType(archive)

	if atype != NoArchive {
		if f, err := os.Open(archive); err == nil {
			defer f.Close()

			switch atype {
			case TarGz:
				if err := untar(destdir, f, func(path string, info os.FileInfo, err error) error {
					if err == nil {
						return manifest.Append(path, info)
					} else {
						return err
					}
				}); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported archive type")
			}

			f.Close()

			log.Debugf("removing extracted archive: %s", archive)
			return os.Remove(archive)
		} else {
			return err
		}
	} else {
		return nil
	}
}

func untar(dst string, r io.Reader, fn filepath.WalkFunc) error {
	if gzr, err := gzip.NewReader(r); err == nil {
		defer gzr.Close()

		tr := tar.NewReader(gzr)

		for {
			header, err := tr.Next()

			switch {
			case err == io.EOF: // if no more files are found return
				return nil
			case err != nil: // return any other error
				return err
			case header == nil: // if the header is nil, just skip it (not sure how this happens)
				continue
			}

			// the target location where the dir/file should be created
			target := filepath.Join(dst, header.Name)

			// check the file type
			switch header.Typeflag {
			case tar.TypeDir: // if its a dir and it doesn't exist create it
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}

			case tar.TypeReg: // if it's a file create it
				if f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode)); err == nil {
					defer f.Close()

					if _, err := io.Copy(f, tr); err != nil {
						return err
					}

					f.Close()
				} else {
					return err
				}
			}

			if fn != nil {
				info, serr := os.Stat(target)

				if err := fn(target, info, serr); err != nil {
					return err
				}
			}
		}
	} else {
		return err
	}
}
