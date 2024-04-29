package util

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
)

func LoadEnvFile(filePath string) {
	if err := godotenv.Load(filePath); err != nil {
		panic(errors.Wrap(err, "read .env file failed"))
	}
}

func GetRequireEnvStringSlice(env string) []string {
	envString := os.Getenv(env)
	if envString == "" {
		panic("no set env: " + env)
	}
	envStringSlice := strings.Split(envString, ",")
	return envStringSlice
}

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

func GetEnvFloat64(env string, fallback float64) float64 {
	envString := os.Getenv(env)
	envFloat64, err := strconv.ParseFloat(envString, 64)
	if err != nil {
		return fallback
	}
	return envFloat64
}
