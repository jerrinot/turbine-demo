package internal

import (
	"os"
	"strconv"
)

func LookupEnvOrBoolean(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		ret, err := strconv.ParseBool(val)
		return err == nil && ret
	}
	return defaultVal
}

func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func Int32Ptr(i int32) *int32 { return &i }
