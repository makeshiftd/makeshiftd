package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

func loadConfig(c interface{}, paths ...string) (string, error) {
	var attempted []string
	for _, path := range paths {
		path = os.ExpandEnv(path)
		abspath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		attempted = append(attempted, abspath)
		data, rerr := ioutil.ReadFile(abspath)
		if rerr != nil {
			continue
		}
		uerr := json.Unmarshal(data, c)
		if uerr != nil {
			return "", uerr
		}
		return abspath, nil
	}
	return "", fmt.Errorf("config not found: %s", strings.Join(attempted, ", "))
}

func saveConfig(config interface{}, path string) error {
	data, merr := json.MarshalIndent(config, "", "    ")
	if merr != nil {
		return merr
	}
	werr := ioutil.WriteFile(path, data, 0664)
	if werr != nil {
		return werr
	}
	return nil
}

type DocRoot struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type AppConfig struct {
	sync.RWMutex
	loadPath string
	Version  float64   `json:"version"`
	Roots    []DocRoot `json:"roots"`
}

func LoadAppConfig(paths ...string) (*AppConfig, error) {
	c := &AppConfig{}
	loadPath, err := loadConfig(c, paths...)
	if err != nil {
		return nil, err
	}
	c.loadPath = loadPath
	return c, nil
}

func (c *AppConfig) Save() error {
	c.RLock()
	defer c.RUnlock()
	return saveConfig(c, c.loadPath)
}

type Timer struct {
	Path string `json:"path"`
}

type ModuleConfig struct {
	sync.RWMutex
	loadPath string
	Timers   []Timer `json:"timers"`
}

func LoadModuleConfig(paths ...string) (*ModuleConfig, error) {
	c := &ModuleConfig{}
	loadPath, err := loadConfig(c, paths...)
	if err != nil {
		return nil, err
	}
	c.loadPath = loadPath
	return c, nil
}

func (c *ModuleConfig) Save() error {
	c.RLock()
	defer c.RUnlock()
	return saveConfig(c, c.loadPath)
}
