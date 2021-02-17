package main

import "github.com/spf13/viper"

// Set the application defaults
func init() {
	viper.SetDefault("server", map[string]interface{}{
		"port": 8080,
		"host": "",
	})
}
