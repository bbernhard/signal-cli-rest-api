package utils

import (
	"os"
	"strconv"
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
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
