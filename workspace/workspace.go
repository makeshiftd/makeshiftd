package workspace

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

	"github.com/makeshiftd/makeshiftd/context"
	"github.com/makeshiftd/makeshiftd/loggers"
	"github.com/makeshiftd/makeshiftd/urlpath"
)

var log = loggers.NewLazyLoggerPkg("workspace")

type Makeshiftd interface {
	Workspaces() []*Workspace
	ServeError(cause interface{}, res http.ResponseWriter, req *http.Request)
}

// Workspace is the handler for a docuemnt root
type Workspace struct {
	Name string
	Slug string
	Root string

	m      Makeshiftd
	err    error
	ctx    context.C
	cancel context.CancelFunc
}

// New creates a new workspace for the given Makeshitfd service
func New(m Makeshiftd, name, root string) *Workspace {
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

func (w *Workspace) ServeHTTP(res http.ResponseWriter, req *http.Request) {

	// ctx, cancel := context.Merge(req.Context(), w.ctx)
	// defer cancel()

	exec := false
	path := req.URL.Path
	log.Debug().Msgf("Serve HTTP slug: %s, path: %s", w.Slug, path)

	segment := ""
	segments := []string{}
	for {
		segment, path = urlpath.PopLeft(path)
		if segment == "" {
			break
		}
		switch {
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
			w.serveError(http.StatusNotFound, res, req)
			return

		case strings.HasPrefix(segment, "!"):
			segment = segment[1:]
			exec = true
		}
		segments = append(segments, segment)
	}

	//segments = append([]string{w.Root}, segments...)
	req.URL.Path = path
	docPath := urlpath.Join(segments...)
	if exec {
		w.execDoc(docPath, res, req)
	} else {
		w.serveDoc(docPath, res, req)
	}
}

func (w *Workspace) serveError(cause interface{}, res http.ResponseWriter, req *http.Request) {
	w.m.ServeError(cause, res, req)
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

func (w *Workspace) execDoc(docPath string, res http.ResponseWriter, req *http.Request) {
	log.Debug().Msgf("Exec file path: %s", docPath)

	var exeDocPath string
	var exeCommand string
	var exeArguments []string
	for _, executer := range executers {
		exeDocPath := docPath + executer.Ext
		exeDocInfo, err := os.Stat(exeDocPath)
		if err != nil && !os.IsNotExist(err) {
			w.serveError(http.StatusInternalServerError, res, req)
			return
		}
		if exeDocInfo.IsDir() {
			continue
		}
		exeCommand = executer.Cmd
		exeArguments = executer.Args[:]
	}
	if exeCommand == "" {
		w.serveError(http.StatusNotFound, res, req)
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
		w.serveError(http.StatusInternalServerError, res, req)
		return
	}
	stderr := &bytes.Buffer{}
	go func() {
		io.Copy(stderr, stderrPipe)
	}()
	// TODO Work Group!
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		w.serveError(http.StatusInternalServerError, res, req)
		return
	}
	stdout := &bytes.Buffer{}
	go func() {
		io.Copy(stdout, stdoutPipe)
	}()

	err = cmd.Start()
	if err != nil {
		w.serveError(http.StatusInternalServerError, res, req)
		return
	}

	err = cmd.Wait()
	if err != nil {
		w.serveError(http.StatusInternalServerError, res, req)
		return
	}

	res.Header().Set("Content-Length", strconv.Itoa(stdout.Len()))
	res.Header().Set("Content-Type", contentType)
	res.WriteHeader(http.StatusOK)
	io.Copy(res, stdout)
}
