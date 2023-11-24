package util

import (
	"os"
	"strconv"
)

func GetEnvString(env, fallback string) string {
	envString := os.Getenv(env)
	if envString == "" {
		return fallback
	}
	return envString
}

func GetEnvBool(env string, fallback bool) bool {
	envString := os.Getenv(env)
	envBool, err := strconv.ParseBool(envString)
	if err != nil {
		return fallback
	}
	return envBool
}
