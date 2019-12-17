package hydra

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ghetzel/go-stockutil/fileutil"
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

func fetch(uri string) (io.ReadCloser, error) {
	var rc io.ReadCloser

	if u, err := url.Parse(uri); err == nil {
		switch u.Scheme {
		case `http`, `https`:
			if res, err := http.Get(u.String()); err == nil {
				if res.StatusCode < 400 {
					rc = res.Body
				} else {
					return nil, fmt.Errorf("http: HTTP %v", res.Status)
				}
			} else {
				return nil, fmt.Errorf("http: %v", err)
			}
		case `file`, ``:
			if f, err := os.Open(fileutil.MustExpandUser(
				filepath.Join(u.Host, u.Path),
			)); err == nil {
				rc = f
			} else {
				return nil, fmt.Errorf("file: %v", err)
			}
		default:
			return nil, fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
	} else {
		return nil, fmt.Errorf("uri: %v", err)
	}

	if rc != nil {
		return rc, nil
	} else {
		return nil, fmt.Errorf("no data")
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
		return fmt.Sprintf("((root.height < root.width) ? (root.height * %f) : (root.width * %f))", f, f)

	} else if strings.HasSuffix(s, `vmax`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vmax`)) / 100.0
		return fmt.Sprintf("((root.height > root.width) ? (root.height * %f) : (root.width * %f))", f, f)

	} else if strings.HasSuffix(s, `vw`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vw`)) / 100.0
		return fmt.Sprintf("(root.width * %f)", f)

	} else if strings.HasSuffix(s, `vh`) {
		f := typeutil.Float(strings.TrimSuffix(s, `vh`)) / 100.0
		return fmt.Sprintf("(root.height * %f)", f)
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
			value = maputil.Apply(value, func(key []string, value interface{}) (interface{}, bool) {
				if qv := qmlstring(value); qv != `` {
					return Literal(qv), true
				} else if vS, ok := value.(string); ok {
					return env(vS), true
				} else {
					return nil, false
				}
			})
		} else if typeutil.IsArray(value) {
			value = sliceutil.Map(value, func(i int, value interface{}) interface{} {
				if qv := qmlstring(value); qv != `` {
					return Literal(qv)
				} else if vS, ok := value.(string); ok {
					return env(vS)
				} else {
					return value
				}
			})
		} else if vS, ok := value.(string); ok {
			value = env(vS)
		}

		// JSONify and return
		if data, err := json.MarshalIndent(value, ``, Indent); err == nil {
			return jsonPostProcess(data)
		} else {
			panic("invalid json: " + err.Error())
		}
	}
}
