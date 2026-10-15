package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env            string
	Port           string
	DBPath         string
	MigrationsPath string // *.sql files are read at runtime
	UploadDir      string
	SessionTTL     time.Duration
	AllowedOrigin  string // for next.js, it handles the CORS and WS CheckOrigins
}

func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Env:            getEnv("APP_ENV", "development"),
		Port:           getEnv("PORT", "8080"),
		DBPath:         getEnv("DB_PATH", "src/server/data/social.db"),
		MigrationsPath: getEnv("MIGRATIONS_PATH", "src/server/pkg/db/migrations/sqlite"),
		UploadDir:      getEnv("UPLOAD_DIR", "src/server/data/uploads"),
		AllowedOrigin:  getEnv("ALLOWED_ORIGIN", "http://localhost:3000"),
	}

	// this is something i personally have never done before in terms of error handling
	// but, here, we're collecting every error that is caused by the config instead of
	// returning them one by one. i guess it would be similar to doing `npm test` where it tests
	// everything and tells you all the errors in one go
	var errs []error

	if cfg.Env != "development" && cfg.Env != "production" {
		errs = append(errs, fmt.Errorf("Error: APP_ENV must be either development or production; %v", cfg.Env))
	}

	if port, err := strconv.Atoi(cfg.Port); err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("Error: port number is invalid; it must be between 1 and 65535; %v", err))
	}

	ttl, err := time.ParseDuration(getEnv("SESSION_TTL", "168h"))
	if err != nil || ttl <= 0 {
		errs = append(errs, fmt.Errorf("Error: SESSION_TTL should be a positive duration instead of %v; %v", os.Getenv("SESSION_TTL"), err))
	}

	// here, we have to convert a time.Duration to time.Time
	//todo, but it also has to be checked later for possible errors, ngl
	cfg.SessionTTL = ttl

	// in case the migrations path doesnt exist
	if info, err := os.Stat(cfg.MigrationsPath); err != nil || !info.IsDir() {
		errs = append(errs, fmt.Errorf("Error: MIGRATIONS_PATH is not a directory.\nTry running the project from the root.\n%v", err))
	}

	return cfg, errors.Join(errs...)
}

// LookupEnv checks for the value depending on the key and returns it if its not empty, else return fallback
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}
