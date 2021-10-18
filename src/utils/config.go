package utils

import (
	"errors"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

type ConfigEntry struct {
	TcpPort      int64  `yaml:"tcp_port"`
	FifoPathname string `yaml:"fifo_pathname"`
}

type Config struct {
	Entries map[string]ConfigEntry `yaml:"config,omitempty"`
}

type JsonRpc2ClientConfig struct {
	config Config
}

func NewJsonRpc2ClientConfig() *JsonRpc2ClientConfig {
	return &JsonRpc2ClientConfig{}
}

func (c *JsonRpc2ClientConfig) Load(path string) error {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, &c.config)
	if err != nil {
		return err
	}

	return nil
}

func (c *JsonRpc2ClientConfig) GetTcpPortForNumber(number string) (int64, error) {
	if val, ok := c.config.Entries[number]; ok {
		return val.TcpPort, nil
	}

	return 0, errors.New("Number " + number + " not found in local map")
}

func (c *JsonRpc2ClientConfig) GetFifoPathnameForNumber(number string) (string, error) {
	if val, ok := c.config.Entries[number]; ok {
		return val.FifoPathname, nil
	}

	return "", errors.New("Number " + number + " not found in local map")
}

func (c *JsonRpc2ClientConfig) GetTcpPortsForNumbers() map[string]int64 {
	mapping := make(map[string]int64)
	for number, val := range c.config.Entries {
		mapping[number] = val.TcpPort
	}

	return mapping
}

func (c *JsonRpc2ClientConfig) AddEntry(number string, configEntry ConfigEntry) {
	if c.config.Entries == nil {
		c.config.Entries = make(map[string]ConfigEntry)
	}
	c.config.Entries[number] = configEntry
}

func (c *JsonRpc2ClientConfig) Persist(path string) error {
	out, err := yaml.Marshal(&c.config)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, out, 0644)
}
