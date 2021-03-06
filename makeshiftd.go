package makeshiftd

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/makeshiftd/makeshiftd/loggers"
	"github.com/makeshiftd/makeshiftd/urlpath"
	"github.com/makeshiftd/makeshiftd/workspace"
)

var log = loggers.NewLazyLoggerPkg("makeshiftd")

// Makeshiftd is the primary handler for the Makeshiftd service
type Makeshiftd struct {
	config        *viper.Viper
	workspaces    []*workspace.Workspace
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
			configFileDir := filepath.Dir(config.ConfigFileUsed())
			root = filepath.Join(configFileDir, root)
		}
		root = filepath.Clean(root)

		w := workspace.New(m, name, root)
		m.workspaces = append(m.workspaces, w)
	}
	return m
}

// Workspaces returns a copy the the configured workspaces
func (m *Makeshiftd) Workspaces() []*workspace.Workspace {
	m.workspacesMtx.RLock()
	defer m.workspacesMtx.RUnlock()
	workspaces := make([]*workspace.Workspace, len(m.workspaces))
	copy(workspaces, m.workspaces)
	return workspaces
}

func (m *Makeshiftd) match(slug string) *workspace.Workspace {
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

func (m *Makeshiftd) ServeError(cause interface{}, res http.ResponseWriter, req *http.Request) {
	code := http.StatusInternalServerError
	msg := http.StatusText(code)
	switch cause := cause.(type) {
	case int:
		code = cause
		msg = http.StatusText(code)
	case error:
		msg = cause.Error()
	}
	res.WriteHeader(code)
	res.Write([]byte(msg))
}

func (m *Makeshiftd) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	log.Debug().Msgf("Serve HTTP path: %s", req.URL.Path)
	slug, path := urlpath.PopLeft(req.URL.Path)
	slug = strings.ToLower(slug)

	if slug == "" {
		m.ServeIndex(res, req)
		return
	}

	w := m.match(slug)
	if w == nil {
		log.Debug().Msgf("Workspace not found: %s", req.URL.Path)
		m.ServeError(http.StatusNotFound, res, req)
		return
	}

	req.URL.Path = path
	w.ServeHTTP(res, req)
}
