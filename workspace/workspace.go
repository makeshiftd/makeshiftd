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
	"sort"
	"strconv"
	"strings"

	"github.com/makeshiftd/makeshiftd/context"
	"github.com/makeshiftd/makeshiftd/loggers"
	"github.com/makeshiftd/makeshiftd/urlpath"
)

var log = loggers.NewLazyLoggerPkg("workspace")

type Makeshiftd interface {
	Workspaces() []*Workspace
	ServeError(res http.ResponseWriter, req *http.Request)
	ServeNotFound(res http.ResponseWriter, req *http.Request)
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
	if err == nil && docInfo.IsDir() {
		pattern := strings.ReplaceAll(docPath, "*", "\\*")
		pattern = strings.ReplaceAll(pattern, "?", "\\?")
		pattern = strings.ReplaceAll(pattern, "[", "\\[")
		pattern = strings.ReplaceAll(pattern, "\\", "\\\\") // Windows?
		pattern = filepath.Join(docPath, "index.*")
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			sort.Strings(matches)
			docPath = matches[0]
			docInfo, err = os.Stat(docPath)
		}
	}
	log.Debug().Msgf("Serve file path: %s", docPath)
	if (err == nil && docInfo.IsDir()) ||
		(err != nil && os.IsNotExist(err)) {
		w.m.ServeNotFound(res, req)
		return
	}
	if err != nil {
		w.m.ServeNotFound(res, req) // TODO 500
		return
	}

	docFile, err := os.Open(docPath)
	if err != nil {
		w.m.ServeNotFound(res, req) // TODO 500
		return
	}
	defer docFile.Close()
	// Disable default redirect by ServeContent().
	if filepath.Base(docPath) == "index.html" {
		docPath = "noredirect.html"
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
	log.Debug().Msgf("Exec file path: %s", docPath)

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
