package utils

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func writeFileAtomic(name string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(name)

	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	tmp := f.Name()
	defer os.Remove(tmp)

	err = f.Chmod(perm)
	if err != nil {
		f.Close()
		return err
	}

	_, err = f.Write(data)
	if err != nil {
		f.Close()
		return err
	}

	err = f.Sync()
	if err != nil {
		f.Close()
		return err
	}

	err = f.Close()
	if err != nil {
		return err
	}

	return os.Rename(tmp, name)
}

type SignalCliTrustMode int

const (
	OnFirstUseTrust SignalCliTrustMode = iota
	AlwaysTrust
	NeverTrust
)

func TrustModeToString(trustMode SignalCliTrustMode) (string, error) {
	if trustMode == OnFirstUseTrust {
		return "on-first-use", nil
	} else if trustMode == AlwaysTrust {
		return "always", nil
	} else if trustMode == NeverTrust {
		return "never", nil
	}
	return "", errors.New("Invalid Trust Mode")
}

func StringToTrustMode(trustMode string) (SignalCliTrustMode, error) {
	if trustMode == "on-first-use" {
		return OnFirstUseTrust, nil
	} else if trustMode == "always" {
		return AlwaysTrust, nil
	} else if trustMode == "never" {
		return NeverTrust, nil
	}
	return OnFirstUseTrust, errors.New("Invalid Trust Mode")
}

type SignalCliApiConfigEntry struct {
	TrustMode SignalCliTrustMode `yaml:"trust_mode"`
}

type SignalCliApiConfigEntries struct {
	Entries map[string]SignalCliApiConfigEntry `yaml:"config,omitempty"`
}

type SignalCliApiConfig struct {
	config SignalCliApiConfigEntries
	path   string
}

func NewSignalCliApiConfig() *SignalCliApiConfig {
	return &SignalCliApiConfig{}
}

func (c *SignalCliApiConfig) Load(path string) error {
	c.path = path
	if _, err := os.Stat(path); err == nil {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		err = yaml.Unmarshal(data, &c.config)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *SignalCliApiConfig) GetTrustModeForNumber(number string) (SignalCliTrustMode, error) {
	if val, ok := c.config.Entries[number]; ok {
		return val.TrustMode, nil
	}

	return NeverTrust, errors.New("Number " + number + " not found in local map")
}

func (c *SignalCliApiConfig) SetTrustModeForNumber(number string, trustMode SignalCliTrustMode) {
	if c.config.Entries == nil {
		c.config.Entries = make(map[string]SignalCliApiConfigEntry)
	}
	c.config.Entries[number] = SignalCliApiConfigEntry{TrustMode: trustMode}
}

func (c *SignalCliApiConfig) Persist() error {
	out, err := yaml.Marshal(&c.config)
	if err != nil {
		return err
	}

	return writeFileAtomic(c.path, out, 0644)
}
