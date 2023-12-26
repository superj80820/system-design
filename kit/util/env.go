package util

import (
	"os"
	"strconv"
)

func GetRequireEnvString(env string) string {
	envString := os.Getenv(env)
	if envString == "" {
		panic("no set env: " + env)
	}
	return envString
}

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

func GetEnvInt(env string, fallback int) int {
	envString := os.Getenv(env)
	envInt, err := strconv.Atoi(envString)
	if err != nil {
		return fallback
	}
	return envInt
}

func GetEnvInt64(env string, fallback int64) int64 {
	envString := os.Getenv(env)
	envInt64, err := strconv.ParseInt(envString, 10, 64)
	if err != nil {
		return fallback
	}
	return envInt64
}
