package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func MustReadFile(t *testing.T, filename string) []byte {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal("Failed to read file: ", filename)
	}
	return data
}

func MustReadAll(t *testing.T, r io.Reader) []byte {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("Failed to read reader")
	}
	return data
}

func MustWriteFile(t *testing.T, filename string, data []byte) {
	dirname := filepath.Dir(filename)
	err := os.MkdirAll(dirname, 0775)
	if err != nil {
		t.Fatal("Failed to make directory: ", dirname)
	}
	err = ioutil.WriteFile(filename, data, 0664)
	if err != nil {
		t.Fatal("Failed to write file: ", filename)
	}
}

func MustRemoveAll(t *testing.T, path string) {
	err := os.RemoveAll(path)
	if err != nil {
		t.Fatal("Failed to remove path: ", path)
	}
}

func AssertEqual(t *testing.T, expect, found, prefix string, args ...interface{}) {
	if expect != found {
		t.Fatalf("%s expecting '%s' found '%s'\n", fmt.Sprintf(prefix, args...), expect, found)
	}
}

func AssertEqualInt(t *testing.T, expect, found int, prefix string, args ...interface{}) {
	if expect != found {
		t.Fatalf("%s expecting '%d' found '%d'\n", fmt.Sprintf(prefix, args...), expect, found)
	}
}

func AssertEqualBytes(t *testing.T, expect, found []byte, prefix string, args ...interface{}) {
	if !bytes.Equal(expect, found) {
		t.Fatalf("%s expecting:\n%s\nfound:\n%s", fmt.Sprintf(prefix, args...), string(expect), string(found))
	}
}

func AssertNotError(t *testing.T, err error, prefix string, args ...interface{}) {
	if err != nil {
		t.Fatalf("%s: %s\n", fmt.Sprintf(prefix, args...), err)
	}
}

func TestAppConfig(t *testing.T) {

	tmpDirPath := filepath.Join("test", "tmp")
	loadPath := filepath.Join(tmpDirPath, "makeshiftd.json")

	var data = []byte(`{
	    "version": 1,
	    "roots": [{
	        "name": "root0",
	        "path": "/path/to/root0"
	    }]
	}`)

	MustWriteFile(t, loadPath, data)
	defer MustRemoveAll(t, tmpDirPath)

	var root0 = DocRoot{
		Name: "root0",
		Path: "/path/to/root0",
	}

	config, err := LoadAppConfig(loadPath)
	AssertNotError(t, err, "Failed to load config file: %s", loadPath)
	AssertEqualInt(t, 1, len(config.Roots), "Config Roots length")
	AssertEqual(t, root0.Name, config.Roots[0].Name, "Config property Roots[0].Name")
	AssertEqual(t, root0.Path, config.Roots[0].Path, "Config property Roots[0].Path")

	root1 := DocRoot{
		Name: "root1",
		Path: "/path/to/root1",
	}
	config.Roots = append(config.Roots, root1)

	err = config.Save()
	AssertNotError(t, err, "Failed to store config file: %s", loadPath)

	config, err = LoadAppConfig(loadPath)
	AssertNotError(t, err, "Failed to reload config file: %s", loadPath)
	AssertEqualInt(t, 2, len(config.Roots), "Config Roots length")
	AssertEqual(t, root1.Name, config.Roots[1].Name, "Config property Roots[1].Name")
	AssertEqual(t, root1.Path, config.Roots[1].Path, "Config property Roots[1].Path")
}
