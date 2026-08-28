package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

/*
Every setting is read once here, at startup, and travels afterwards inside a
Config value. No package calls os.Getenv on its own, so there is a single place
that knows the names of the variables and a single moment where a bad value can
stop the process
*/

const PlaylistFilename = "segment.m3u8"

const (
	envListenAddr      = "LISTEN_ADDR"
	envDatabaseURL     = "DATABASE_URL"
	envSegmentsDir     = "SEGMENTS_DIR"
	envWebDir          = "WEB_DIR"
	envCookieSecure    = "COOKIE_SECURE"
	envTickInterval    = "TICK_INTERVAL"
	envChatHistorySize = "CHAT_HISTORY_SIZE"
)

const (
	defaultListenAddr      = ":8080"
	defaultDatabaseURL     = "postgres://zapping:zapping@localhost:5432/zapping"
	defaultWebDir          = "web"
	defaultCookieSecure    = true
	defaultTickInterval    = 10 * time.Second
	defaultChatHistorySize = 50
)

type Config struct {
	ListenAddr      string
	DatabaseURL     string
	SegmentsDir     string
	WebDir          string
	SecureCookies   bool
	TickInterval    time.Duration
	ChatHistorySize int
}

/*
Fail fast: a wrong duration or a negative size kills the boot instead of
surfacing later as a stream that never advances
*/
func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:  getenv(envListenAddr, defaultListenAddr),
		DatabaseURL: getenv(envDatabaseURL, defaultDatabaseURL),
		WebDir:      getenv(envWebDir, defaultWebDir),
	}

	/*
		The only setting with no default. Every other one names something this
		repository ships or creates, so a fallback is always meaningful; the media
		is half a gigabyte of external input that lives wherever the person running
		this unpacked it. A default would only be a path that happens to be wrong,
		failing later and less clearly than saying so here
	*/
	cfg.SegmentsDir = os.Getenv(envSegmentsDir)
	if cfg.SegmentsDir == "" {
		return nil, fmt.Errorf("%s is required: point it at the directory holding segment.m3u8 and the .ts files it names", envSegmentsDir)
	}

	secure, err := getenvBool(envCookieSecure, defaultCookieSecure)
	if err != nil {
		return nil, err
	}
	cfg.SecureCookies = secure

	tick, err := getenvDuration(envTickInterval, defaultTickInterval)
	if err != nil {
		return nil, err
	}
	if tick <= 0 {
		return nil, fmt.Errorf("%s must be positive, got %s", envTickInterval, tick)
	}
	cfg.TickInterval = tick

	size, err := getenvInt(envChatHistorySize, defaultChatHistorySize)
	if err != nil {
		return nil, err
	}
	if size <= 0 {
		return nil, fmt.Errorf("%s must be positive, got %d", envChatHistorySize, size)
	}
	cfg.ChatHistorySize = size

	return cfg, nil
}

/*
os.Getenv cannot tell "unset" from "set to empty", and for every one of these
settings an empty string is not a usable value, so both cases take the default
*/
func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) (bool, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func getenvInt(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}

func getenvDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return parsed, nil
}
