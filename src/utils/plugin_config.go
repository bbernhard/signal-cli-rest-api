package utils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type PluginConfig struct {
	Endpoint   string `yaml:"endpoint"`
	Method     string `yaml:"method"`
	Version    int    `yaml:"version,omitempty"`
	ScriptPath string
}

func NewPluginConfigs() *PluginConfigs {
	return &PluginConfigs{}
}

type PluginConfigs struct {
	Configs []PluginConfig
}

func (c *PluginConfigs) Load(baseDirectory string) error {

	err := filepath.Walk(baseDirectory, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".def" {
			return nil
		}

		if _, err := os.Stat(path); err == nil {
			data, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			var pluginConfig PluginConfig
			pluginConfig.Version = 1
			err = yaml.Unmarshal(data, &pluginConfig)
			if err != nil {
				return err
			}
			pluginConfig.ScriptPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".lua"
			c.Configs = append(c.Configs, pluginConfig)
		}
		return nil
	})

	return err
}
