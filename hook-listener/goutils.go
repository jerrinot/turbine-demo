package main

import (
	"os"
	"strconv"
)

func lookupEnvOrBoolean(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		ret, err := strconv.ParseBool(val)
		return err == nil && ret
	}
	return defaultVal
}

func lookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func int32Ptr(i int32) *int32 { return &i }
