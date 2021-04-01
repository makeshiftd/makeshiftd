package main_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var BaseURL = "http://localhost:8080"

var TestDataPath = filepath.Join("..", "..", "testdata")

func Get(url string) (*http.Response, []byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res, nil, err
	}
	return res, body, nil
}

func Post(url string, data []byte) (*http.Response, []byte, error) {
	return Request("POST", url, data)
}

func Put(url string, data []byte) (*http.Response, []byte, error) {
	return Request("PUT", url, data)
}

func Request(method, url string, data []byte) (*http.Response, []byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res, nil, err
	}
	return res, body, nil
}

func TestMakeShiftdIndex(t *testing.T) {
	res, body, err := Get(BaseURL)
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, []byte("Makeshiftd"), body)
}

func TestGetIndex(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/index.html")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}

func TestGetPage(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1/page.html")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/page.html")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}

func TestGetJSON(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1/data.json")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "application/json", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/data.json")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}

func TestServeDocPost(t *testing.T) {

	table := []struct {
		Path string
		Code int
	}{
		{
			Path: "/ws1/temp/data.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/data.json",
			Code: http.StatusConflict,
		},
		{
			Path: "/ws1/temp/data_*.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/dir1/data.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/dir2/dir3/data.json",
			Code: http.StatusCreated,
		},
	}

	temp := filepath.Join(TestDataPath, "workspace1", "temp")
	err := os.MkdirAll(temp, os.ModePerm)
	require.Nil(t, err, err)
	defer os.RemoveAll(temp)

	for idx, row := range table {
		t.Run("POST:"+row.Path, func(t *testing.T) {
			data := []byte(fmt.Sprintf(`{ "test": "hello World %d" }`, idx))

			res, body, err := Post(BaseURL+row.Path, data)
			require.Nil(t, err, err)
			require.Equal(t, row.Code, res.StatusCode)

			if row.Code >= 400 {
				return
			}

			location := res.Header.Get("Location")
			if strings.Contains(row.Path, "*") {
				p := regexp.QuoteMeta(row.Path)
				i := strings.LastIndex(p, "\\*")
				p = p[:i] + "\\d+" + row.Path[i+1:]
				require.Regexp(t, regexp.MustCompile(p), location)
			} else {
				require.Equal(t, row.Path, location)
			}

			res, body, err = Get(BaseURL + location)
			require.Nil(t, err, err)
			require.Equal(t, http.StatusOK, res.StatusCode)

			require.Equalf(t, data, body, "body:%s", string(body))
		})
	}
}

func TestServeDocPut(t *testing.T) {

	table := []struct {
		Path string
		Code int
	}{
		{
			Path: "/ws1/temp/data.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/data.json",
			Code: http.StatusOK,
		},
		{
			Path: "/ws1/temp/data_*.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/dir1/data.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/dir2/dir3/data.json",
			Code: http.StatusCreated,
		},
		{
			Path: "/ws1/temp/dir1/data.json",
			Code: http.StatusOK,
		},
		{
			Path: "/ws1/temp/dir2/dir3",
			Code: http.StatusConflict,
		},
	}

	temp := filepath.Join(TestDataPath, "workspace1", "temp")
	err := os.MkdirAll(temp, os.ModePerm)
	require.Nil(t, err, err)
	defer os.RemoveAll(temp)

	for idx, row := range table {
		data := []byte(fmt.Sprintf(`{ "test": "hello World %d" }`, idx))

		t.Run("PUT:"+row.Path, func(t *testing.T) {
			res, body, err := Put(BaseURL+row.Path, data)
			require.Nil(t, err, err)
			require.Equal(t, row.Code, res.StatusCode)

			if row.Code >= 400 {
				return
			}

			res, body, err = Get(BaseURL + row.Path)
			require.Nil(t, err, err)
			require.Equal(t, http.StatusOK, res.StatusCode)

			require.Equalf(t, data, body, "body:%s", string(body))
		})
	}
}
