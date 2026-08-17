package utils

import (
	"io"
	"io/fs"
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
	baseDirectory = filepath.Clean(baseDirectory)
	root, err := os.OpenRoot(baseDirectory)
	if err != nil {
		return err
	}
	defer root.Close()

	err = filepath.WalkDir(baseDirectory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".def" {
			return nil
		}

		relPath, err := filepath.Rel(baseDirectory, path)
		if err != nil {
			return err
		}

		f, err := root.Open(relPath)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}

		var pluginConfig PluginConfig
		pluginConfig.Version = 1 // default; overridden by yaml if present
		if err = yaml.Unmarshal(data, &pluginConfig); err != nil {
			return err
		}
		pluginConfig.ScriptPath = strings.TrimSuffix(path, filepath.Ext(path)) + ".lua"
		c.Configs = append(c.Configs, pluginConfig)
		return nil
	})
	return err
}