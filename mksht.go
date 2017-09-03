package main

import (
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/xyplane/debugger"
)

var debug = debugger.Debug("makeshiftd")

type MkShtHandler struct {
	Config   *AppConfig
	Handlers []*DocRootHandler
}

func NewMkShtHanlder(config *AppConfig) *MkShtHandler {
	h := &MkShtHandler{
		Config: config,
	}
	h.Reload()
	return h
}

func (h *MkShtHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	segs := strings.SplitN(req.URL.Path, "/", 3)
	if len(segs) < 2 {
		http.NotFound(res, req)
		return
	}

	prefix := "/" + segs[1]

	if len(segs) == 3 {
		dirtyPath := "/" + segs[2]
		cleanPath := path.Clean(dirtyPath)
		if dirtyPath != cleanPath {
			http.Redirect(res, req, prefix+cleanPath, http.StatusPermanentRedirect)
			return
		}
		req.URL.Path = cleanPath
	} else {
		req.URL.Path = "/"
	}

	prefix = strings.ToLower(prefix)

	for _, rh := range h.Handlers {
		if rh.Prefix == prefix {
			rh.ServeHTTP(res, req)
			return
		}
	}

	http.NotFound(res, req)
}

func (h *MkShtHandler) Reload() {
	// TODO: shutdown
	h.Handlers = nil
	for _, root := range h.Config.Roots {
		var abspath string
		if filepath.IsAbs(root.Path) {
			abspath = root.Path
		} else {
			loadDir := filepath.Dir(h.Config.loadPath)
			abspath = filepath.Join(loadDir, root.Path)
		}
		abspath = filepath.Clean(abspath)
		rh := &DocRootHandler{
			Prefix: "/" + strings.ToLower(root.Name),
			Path:   abspath,
		}
		h.Handlers = append(h.Handlers, rh)
	}
}

type DocRootHandler struct {
	Config *ModuleConfig
	Prefix string
	Path   string
}

func (h *DocRootHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	doc := h.Path
	stat, serr := os.Stat(doc)
	if serr != nil || !stat.IsDir() {
		// TODO: 500 Server Error!
		http.NotFound(res, req)
		return
	}
	segs := strings.Split(req.URL.Path, "/")
	// When splitting a path starting with "/",
	// the first element will always be emply.
	segs = segs[1:]
	for len(segs) > 0 {
		seg := segs[0]
		segs = segs[1:]
		if len(seg) == 0 {
			continue
		}
		if seg[0] == '.' || seg[0] == '_' {
			http.NotFound(res, req)
			return
		}
		doc = filepath.Join(doc, seg)
		stat, serr = os.Stat(doc)
		if len(segs) > 0 && (serr != nil || !stat.IsDir()) {
			http.NotFound(res, req)
			return
		}
	}

	if serr == nil && stat.IsDir() {
		doc = filepath.Join(doc, "index.html")
	}

	dir := filepath.Dir(doc)
	stat, serr = os.Stat(filepath.Join(dir, "Makefile"))
	if serr == nil && !stat.IsDir() {
		debug("Execute 'make' command")
		cmd := exec.Command("make")
		cmd.Dir = dir
		output, cerr := cmd.CombinedOutput()
		switch cerr.(type) {
		case *exec.ExitError:
			res.Header().Add("Content-Type", "text/plain")
			res.WriteHeader(http.StatusInternalServerError)
			res.Write(output)
			return
		case *exec.Error:
			res.Header().Add("Content-Type", "test/plain")
			res.WriteHeader(http.StatusInternalServerError)
			res.Write([]byte(`build command 'make' not found`))
			return
		}
	}

	stat, serr = os.Stat(doc)
	if serr != nil || stat.IsDir() {
		debug("Document not found: %s", doc)
		http.NotFound(res, req)
		return
	}
	file, oerr := os.Open(doc)
	if oerr != nil {
		// TODO: 500 Server Error!
		http.NotFound(res, req)
		return
	}
	// Disable default redirect by ServeContent().
	if filepath.Base(doc) == "index.html" {
		doc = "noredirect.html"
	}
	http.ServeContent(res, req, doc, stat.ModTime(), file)
}
