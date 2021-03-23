package main_test

import (
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

var BaseURL = "http://localhost:8080"

var TestDataPath = "../../testdata"

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
		return nil, nil, err
	}
	return res, body, nil
}

func TestIndex(t *testing.T) {
	res, body, err := Get(BaseURL)
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, []byte("Makeshiftd"), body)
}

func TestWSGetIndex(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/index.html")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}

func TestWSGetPage(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1/page.html")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/page.html")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}

func TestWSGetJSON(t *testing.T) {
	res, body, err := Get(BaseURL + "/ws1/data.json")
	require.Nil(t, err, err)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "application/json", res.Header.Get("Content-Type"))

	data, err := os.ReadFile(TestDataPath + "/workspace1/data.json")
	require.Nil(t, err, err)
	require.Equal(t, body, data)
}
