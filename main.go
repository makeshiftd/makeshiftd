package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Configuration file path")
	flag.StringVar(&configPath, "c", "", "See -config")
	flag.Parse()

	var config *AppConfig
	{
		var err error
		if configPath != "" {
			config, err = LoadAppConfig(configPath)
		} else {
			config, err = LoadAppConfig(
				"makeshiftd.json",
				"${HOME}/.makeshiftd.json",
				"/etc/makeshiftd.json",
			)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	mksht := NewMkShtHanlder(config)

	http.Handle("/", mksht)

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
