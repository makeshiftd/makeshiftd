package workspace

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/makeshiftd/makeshiftd/urlpath"
)

func (w *Workspace) serveDoc(docPath string, res http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case "GET":
		w.serveDocGet(docPath, res, req)
	case "POST":
		w.serveDocPost(docPath, res, req)
	case "PUT":
		w.serveDocPut(docPath, res, req)
	case "PATCH":
		// serveDocPatch(docPath, res, req)
	default:
		w.serveError(http.StatusMethodNotAllowed, res, req)
	}
}

func (w *Workspace) serveDocGet(docPath string, res http.ResponseWriter, req *http.Request) {

	docFilePath := filepath.FromSlash(docPath)
	docFilePath = filepath.Join(w.Root, docFilePath)

	docFileInfo, err := os.Stat(docFilePath)
	if err == nil && docFileInfo.IsDir() {
		pattern := strings.ReplaceAll(docFilePath, "*", "\\*")
		pattern = strings.ReplaceAll(pattern, "?", "\\?")
		pattern = strings.ReplaceAll(pattern, "[", "\\[")
		pattern = strings.ReplaceAll(pattern, "\\", "\\\\") // Windows?
		pattern = filepath.Join(docFilePath, "index.*")
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			// TODO: content nego
			sort.Strings(matches)
			docFilePath = matches[0]
			docFileInfo, err = os.Stat(docFilePath)
		}
	}
	log.Debug().Msgf("Get file path: %s", docFilePath)
	if (err == nil && docFileInfo.IsDir()) ||
		(err != nil && os.IsNotExist(err)) {
		w.serveError(http.StatusNotFound, res, req)
		return
	}
	if err != nil {
		w.serveError(err, res, req)
		return
	}

	docFile, err := os.Open(docFilePath)
	if err != nil {
		w.serveError(err, res, req)
		return
	}
	defer docFile.Close()
	// Disable default redirect by ServeContent().
	if filepath.Base(docPath) == "index.html" {
		docFilePath = "noredirect.html"
	}
	http.ServeContent(res, req, docFilePath, docFileInfo.ModTime(), docFile)
}

func (w *Workspace) serveDocPost(docPath string, res http.ResponseWriter, req *http.Request) {
	docDir, docName := urlpath.Split(docPath)

	docFileDir := filepath.FromSlash(docDir)
	docFileDir = filepath.Join(w.Root, docFileDir)
	docFilePath := filepath.Join(docFileDir, docName)
	log.Debug().Msgf("Post file path: %s", docFilePath)

	mktemp := strings.Contains(docName, "*")
	if !mktemp {
		_, err := os.Stat(docFilePath)
		if err != nil && !os.IsNotExist(err) {
			w.serveError(err, res, req)
			return
		}
		if err == nil {
			w.serveError(http.StatusConflict, res, req)
			return
		}
	}

	err := os.MkdirAll(docFileDir, os.ModePerm)
	if err != nil {
		w.serveError(err, res, req)
		return
	}

	var docFile *os.File
	if mktemp {
		docFile, err = os.CreateTemp(docFileDir, docName)
	} else {
		docFile, err = os.Create(docFilePath)
	}
	if err != nil {
		w.serveError(err, res, req)
		return
	}
	defer docFile.Close()

	nbytes, err := io.Copy(docFile, req.Body)
	if err != nil {
		log.Err(err).Msgf("Error copying request body to file")
		w.serveError(err, res, req)
		return
	}
	log.Trace().Msgf("Request body copied to file: %d bytes", nbytes)

	req.Body.Close()

	docName = filepath.Base(docFile.Name())
	location := urlpath.Join("/", w.Slug, docDir, docName)
	res.Header().Add("Location", location)
	res.WriteHeader(http.StatusCreated)
}

func (w *Workspace) serveDocPut(docPath string, res http.ResponseWriter, req *http.Request) {
	docDir, docName := urlpath.Split(docPath)

	docFileDir := filepath.FromSlash(docDir)
	docFileDir = filepath.Join(w.Root, docFileDir)
	docFilePath := filepath.Join(docFileDir, docName)
	log.Debug().Msgf("Put file path: %s", docFilePath)

	docFileExists := true
	docFileInfo, err := os.Stat(docFilePath)
	if err != nil && !os.IsNotExist(err) {
		w.serveError(err, res, req)
		return
	}
	if err == nil && docFileInfo.IsDir() {
		w.serveError(http.StatusConflict, res, req)
		return
	}

	if err != nil {
		docFileExists = false
		err := os.MkdirAll(docFileDir, os.ModePerm)
		if err != nil {
			w.serveError(err, res, req)
			return
		}
	}

	docFile, err := os.Create(docFilePath)
	if err != nil {
		w.serveError(err, res, req)
		return
	}
	defer docFile.Close()

	nbytes, err := io.Copy(docFile, req.Body)
	if err != nil {
		log.Err(err).Msgf("Error copying request body to file")
		w.serveError(err, res, req)
		return
	}
	log.Trace().Msgf("Request body copied to file: %d bytes", nbytes)

	req.Body.Close()

	if docFileExists {
		res.WriteHeader(http.StatusOK)
	} else {
		res.WriteHeader(http.StatusCreated)
	}
}
