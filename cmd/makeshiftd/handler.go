package main

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/makeshiftd/makeshiftd/context"
	"github.com/spf13/viper"
)

// Makeshiftd is the primary handler for the Makeshiftd service
type Makeshiftd struct {
	config *viper.Viper

	workspaces    []*Workspace
	workspacesMtx sync.RWMutex
}

// New creates a new Makeshiftd service from the configuration
func New(config *viper.Viper) *Makeshiftd {
	m := &Makeshiftd{
		config: config,
	}

	wsconfig := config.GetStringMapString("workspaces")
	for name, root := range wsconfig {
		if !filepath.IsAbs(root) {
			root = filepath.Join(root, config.ConfigFileUsed())
		}
		root = filepath.Clean(root)

		w := NewWorkspace(m, name, root)
		m.workspaces = append(m.workspaces, w)
	}
	return m
}

// Workspaces returns a copy the the configured workspaces
func (m *Makeshiftd) Workspaces() []*Workspace {
	m.workspacesMtx.RLock()
	defer m.workspacesMtx.RUnlock()
	workspaces := make([]*Workspace, len(m.workspaces))
	copy(workspaces, m.workspaces)
	return workspaces
}

func (m *Makeshiftd) match(slug string) *Workspace {
	m.workspacesMtx.RLock()
	defer m.workspacesMtx.RUnlock()

	for _, ws := range m.workspaces {
		if slug == ws.Slug {
			return ws
		}
	}
	return nil
}

func (m *Makeshiftd) ServeIndex(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("Makeshiftd"))
}

func (m *Makeshiftd) ServeNotFound(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotFound)
	res.Write([]byte("Not Found"))
}

func (m *Makeshiftd) ServeError(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotFound)
	res.Write([]byte("Internal Server Error"))
}

// func splitSlug(path string) (string, string) {
// 	idx := strings.Index(path, "/")
// 	if idx < -1 {
// 		return strings.ToLower(path), "/"
// 	}
// 	if idx > 0 {
// 		return strings.ToLower(path[:idx]), path[idx+1:]
// 	}
// 	return splitSlug(path[1:])
// }

func (m *Makeshiftd) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	slug, path := popPath(req.URL.Path)
	slug = strings.ToLower(slug)

	if slug == "" {
		m.ServeIndex(res, req)
		return
	}

	w := m.match(slug)
	if w == nil {
		log.Debug().Msgf("Workspace not found: %s", req.URL.Path)
		m.ServeNotFound(res, req)
		return
	}

	req.URL.Path = path
	w.ServeHTTP(res, req)
}

// Workspace is the handler for a docuemnt root
type Workspace struct {
	Name string
	Slug string
	Root string

	m      *Makeshiftd
	err    error
	ctx    context.C
	cancel context.CancelFunc
}

// NewWorkspace creates a new workspace for the given Makeshitfd service
func NewWorkspace(m *Makeshiftd, name, root string) *Workspace {
	ctx, cancel := context.WithCancel(context.Background())

	slug := strings.ToLower(name)

	w := &Workspace{
		Name:   name,
		Slug:   slug,
		Root:   root,
		m:      m,
		ctx:    ctx,
		cancel: cancel,
	}

	for _, workspace := range m.Workspaces() {
		if slug == workspace.Slug {
			w.err = fmt.Errorf("Workspace slug is not unique")
		}
	}

	if info, err := os.Stat(root); err == nil {
		if !info.IsDir() {
			w.err = fmt.Errorf("Workspace root is not a directory")
		}
	} else {
		w.err = fmt.Errorf("Workspace root not found")
	}

	return w
}

// Cancel cancels this workspace and all associated requests
func (w *Workspace) Cancel() {
	w.cancel()
}

func popPath(p string) (string, string) {
	idx := strings.Index(p, "/")
	if idx < 0 {
		return p, ""
	}
	if idx > 0 {
		return p[:idx-1], p[idx+1:]
	}
	startIdx := 0
	for idx == 0 {
		startIdx++
		idx = strings.Index(p[startIdx:], "/")
	}
	if idx < 0 {
		return p[startIdx:], ""
	}
	return p[startIdx : startIdx+idx], p[startIdx+idx:]
}

func (w *Workspace) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	// ctx, cancel := context.Merge(req.Context(), w.ctx)
	// defer cancel()
	res.WriteHeader(http.StatusOK)
	res.Write([]byte("Hello from Workspace"))

	exec := false
	path := req.URL.Path

	segment := ""
	segments := []string{}
	for {
		if path == "" {
			break
		}

		if path == "/" {
			segments = append(segments, "index.html")
			break
		}

		segment, path = popPath(path)
		switch {
		// case segment == "":
		// 	break

		case segment == ".":
			continue

		case segment == "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
				continue
			}
			fallthrough
		case strings.HasPrefix(segment, "."):
			fallthrough
		case strings.HasPrefix(segment, "_"):
			w.m.ServeNotFound(res, req)
			return

		case strings.HasPrefix(segment, "!"):
			segment = segment[1:]
			exec = true
		}
		segments = append(segments, segment)
	}

	segments = append([]string{w.Root}, segments...)

	req.URL.Path = path
	docPath := filepath.Join(segments...)
	if exec {
		w.execPath(docPath, res, req)
	} else {
		w.servePath(docPath, res, req)
	}
}

func (w *Workspace) servePath(docPath string, res http.ResponseWriter, req *http.Request) {

	docInfo, err := os.Stat(docPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.m.ServeNotFound(res, req)
		} else {
			w.m.ServeNotFound(res, req) // TODO 500
		}
		return
	}
	if docInfo.IsDir() {
		w.m.ServeNotFound(res, req)
		return
	}
	docFile, err := os.Open(docPath)
	if err != nil {
		// 500
		return
	}
	http.ServeContent(res, req, docPath, docInfo.ModTime(), docFile)
}

var executers = []struct {
	Ext  string
	Env  string
	Cmd  string
	Args []string
}{
	{
		Ext:  ".go",
		Cmd:  "go",
		Args: []string{"run"},
	},
}

func (w *Workspace) execPath(docPath string, res http.ResponseWriter, req *http.Request) {

	var exeDocPath string
	var exeCommand string
	var exeArguments []string
	for _, executer := range executers {
		exeDocPath := docPath + executer.Ext
		exeDocInfo, err := os.Stat(exeDocPath)
		if err != nil && !os.IsNotExist(err) {
			w.m.ServeError(res, req)
			return
		}
		if exeDocInfo.IsDir() {
			continue
		}
		exeCommand = executer.Cmd
		exeArguments = executer.Args[:]
	}
	if exeCommand == "" {
		w.m.ServeNotFound(res, req)
		return
	}

	contentType := mime.TypeByExtension(filepath.Ext(docPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	exeArguments = append(exeArguments, exeDocPath)

	cmd := exec.CommandContext(req.Context(), exeCommand, exeArguments...)

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		w.m.ServeError(res, req)
		return
	}
	stderr := &bytes.Buffer{}
	go func() {
		io.Copy(stderr, stderrPipe)
	}()
	// TODO Work Group!
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		w.m.ServeError(res, req)
		return
	}
	stdout := &bytes.Buffer{}
	go func() {
		io.Copy(stdout, stdoutPipe)
	}()

	err = cmd.Start()
	if err != nil {
		w.m.ServeError(res, req)
		return
	}

	err = cmd.Wait()
	if err != nil {
		w.m.ServeError(res, req)
		return
	}

	res.Header().Set("Content-Length", strconv.Itoa(stdout.Len()))
	res.Header().Set("Content-Type", contentType)
	res.WriteHeader(http.StatusOK)
	io.Copy(res, stdout)
}
