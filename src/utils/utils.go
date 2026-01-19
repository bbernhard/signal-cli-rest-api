package utils

import (
	"os"
	"strconv"
	"errors"
	log "github.com/sirupsen/logrus"
)

func GetEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func GetIntEnv(key string, defaultVal int) (int, error) {
	if value, exists := os.LookupEnv(key); exists {
		intValue, err := strconv.Atoi(value)
		if err != nil {
			return 0, err
		}
		return intValue, nil
	}
	return defaultVal, nil
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func IsPhoneNumber(s string) bool {
	for index, c := range s {
		if index == 0 {
			if c != '+' {
				return false
			}
		} else {
			if (c < '0' || c > '9') && (c != ' ') {
				return false
			}
		}
	}
	return true
}

func SetLogLevel(logLevel string) error {
	if logLevel == "debug" {
		log.SetLevel(log.DebugLevel)
	} else if logLevel == "info" {
		log.SetLevel(log.InfoLevel)
	} else if logLevel == "error" {
		log.SetLevel(log.ErrorLevel)
	} else if logLevel == "warn" {
		log.SetLevel(log.WarnLevel)
	} else {
		return errors.New("Couldn't set log level - invalid log level")
	}
	return nil
}
