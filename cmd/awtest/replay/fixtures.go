// Package replay serves canned AWS API responses from fixture files on a
// loopback-only HTTP server so awtest can run a complete scan offline.
package replay

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// maxFixtureSize is the largest fixture file that will be loaded.
const maxFixtureSize = 5 << 20

var (
	regionRe = regexp.MustCompile(`^[a-z]{2}(-[a-z]+)+-[0-9]+$`)
	statusRe = regexp.MustCompile(`^(.+)\.([0-9]{3})$`)
)

type key struct {
	service, region, action, resource string
}

type fixture struct {
	status      int
	contentType string
	body        []byte
	file        string
}

// Fixtures is an in-memory set of canned responses.
type Fixtures struct {
	m map[key]*fixture
}

// Len returns the number of loaded fixtures.
func (f *Fixtures) Len() int { return len(f.m) }

// lookup finds the fixture for a request. Region-specific fixtures win over
// generic ones, and resource-specific over action-wide ones.
func (f *Fixtures) lookup(service, region, action, resource string) (*fixture, bool) {
	var cands []key
	if region != "" && resource != "" {
		cands = append(cands, key{service, region, action, resource})
	}
	if region != "" {
		cands = append(cands, key{service, region, action, ""})
	}
	if resource != "" {
		cands = append(cands, key{service, "", action, resource})
	}
	cands = append(cands, key{service, "", action, ""})
	for _, k := range cands {
		if fx, ok := f.m[k]; ok {
			return fx, true
		}
	}
	return nil, false
}

// Load reads every .xml/.json regular file in fsys into memory. Layouts:
//
//	<service>/<Action>.<ext>
//	<service>/<region>/<Action>.<ext>
//	<service>/<Action>/<resource>.<ext>
//	<service>/<region>/<Action>/<resource>.<ext>
//
// A file name may carry a status suffix before the extension
// (<name>.404.xml); the default status is 200. Other files, symlinks and
// directories are skipped.
func Load(fsys fs.FS) (*Fixtures, error) {
	f := &Fixtures{m: map[key]*fixture{}}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		ext := strings.ToLower(path.Ext(p))
		if ext != ".xml" && ext != ".json" {
			return nil
		}
		k, status, err := parsePath(p)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("fixture %s: %w", p, err)
		}
		if info.Size() > maxFixtureSize {
			return fmt.Errorf("fixture %s: larger than %d bytes", p, maxFixtureSize)
		}
		fh, err := fsys.Open(p)
		if err != nil {
			return fmt.Errorf("fixture %s: %w", p, err)
		}
		defer fh.Close()
		body, err := io.ReadAll(io.LimitReader(fh, maxFixtureSize+1))
		if err != nil {
			return fmt.Errorf("fixture %s: %w", p, err)
		}
		if len(body) > maxFixtureSize {
			return fmt.Errorf("fixture %s: larger than %d bytes", p, maxFixtureSize)
		}
		if prev, dup := f.m[k]; dup {
			return fmt.Errorf("duplicate fixture for %s: %s and %s", k.describe(), prev.file, p)
		}
		ct := "text/xml"
		if ext == ".json" {
			ct = "application/x-amz-json-1.1"
		}
		f.m[k] = &fixture{status: status, contentType: ct, body: body, file: p}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (k key) describe() string {
	s := k.service
	if k.region != "" {
		s += "/" + k.region
	}
	s += ":" + k.action
	if k.resource != "" {
		s += "/" + k.resource
	}
	return s
}

func parsePath(p string) (key, int, error) {
	parts := strings.Split(p, "/")
	last := parts[len(parts)-1]
	base := strings.TrimSuffix(last, path.Ext(last))
	status := 200
	if m := statusRe.FindStringSubmatch(base); m != nil {
		status, _ = strconv.Atoi(m[2])
		base = m[1]
	}
	if base == "" || status < 100 || status > 599 {
		return key{}, 0, fmt.Errorf("fixture %s: invalid file name", p)
	}
	k := key{service: parts[0]}
	switch len(parts) {
	case 2:
		k.action = base
	case 3:
		if regionRe.MatchString(parts[1]) {
			k.region, k.action = parts[1], base
		} else {
			k.action, k.resource = parts[1], base
		}
	case 4:
		if !regionRe.MatchString(parts[1]) {
			return key{}, 0, fmt.Errorf("fixture %s: %q is not a region", p, parts[1])
		}
		k.region, k.action, k.resource = parts[1], parts[2], base
	default:
		return key{}, 0, fmt.Errorf("fixture %s: unexpected layout (see the replay section of the README)", p)
	}
	return k, status, nil
}
