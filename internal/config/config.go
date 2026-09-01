// Package config provides support for loading the application configuration.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Env   string `env:"APP_ENV" envDefault:"dev"`
	Debug bool   `env:"DEBUG" envDefault:"true"`
	HTTP  HTTP   `envPrefix:"HTTP_"`
	DB    DB     `envPrefix:"DB_"`
	OTel  OTel   `envPrefix:"OTEL_"`
	Auth  Auth   `envPrefix:"AUTH_"`
}

type HTTP struct {
	Addr            string        `env:"ADDR" envDefault:":8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"35s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" envDefault:"120s"`
	RequestTimeout  time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s"`
	MaxBodyBytes    int64         `env:"MAX_BODY_BYTES" envDefault:"1048576"`

	// CORSAllowedOrigins enables CORS when set, e.g. https://app.example.com.
	CORSAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS"`
}

type OTel struct {
	Enabled      bool    `env:"ENABLED" envDefault:"false"`
	ServiceName  string  `env:"SERVICE_NAME" envDefault:"anvil"`
	OTLPEndpoint string  `env:"EXPORTER_OTLP_ENDPOINT" envDefault:"localhost:4317"`
	OTLPInsecure bool    `env:"EXPORTER_OTLP_INSECURE" envDefault:"true"`
	SampleRatio  float64 `env:"SAMPLE_RATIO" envDefault:"1.0"`
}

type Auth struct {
	Secret   string        `env:"SECRET,required"`
	TokenTTL time.Duration `env:"TOKEN_TTL" envDefault:"15m"`
}

type DB struct {
	DSN                string        `env:"DSN,required"`
	Migrate            bool          `env:"MIGRATE" envDefault:"true"`
	MaxOpenConns       int           `env:"MAX_OPEN_CONNS" envDefault:"10"`
	MaxIdleConns       int           `env:"MAX_IDLE_CONNS" envDefault:"5"`
	ConnMaxLifetime    time.Duration `env:"CONN_MAX_LIFETIME" envDefault:"1h"`
	ConnMaxIdleTime    time.Duration `env:"CONN_MAX_IDLE_TIME" envDefault:"30m"`
	SlowQueryThreshold time.Duration `env:"SLOW_QUERY_THRESHOLD" envDefault:"200ms"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parsing environment: %w", err)
	}

	if cfg.HTTP.WriteTimeout <= cfg.HTTP.RequestTimeout {
		return Config{}, fmt.Errorf(
			"HTTP_WRITE_TIMEOUT (%s) must be greater than HTTP_REQUEST_TIMEOUT (%s)",
			cfg.HTTP.WriteTimeout, cfg.HTTP.RequestTimeout,
		)
	}

	return cfg, nil
}
