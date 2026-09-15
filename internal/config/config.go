// Package config contains application configuration parameters.
package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/dronm/webapp"
	websession "github.com/dronm/webapp/session"
)

const DefaultConfigPath = "config.json"

type Config struct {
	HTTP          HTTPConfig          `json:"http"`
	Database      DatabaseConfig      `json:"database"`
	CORS          CORSConfig          `json:"cors"`
	Session       SessionConfig       `json:"session"`
	Debug         DebugConfig         `json:"debug"`
	MainMenu      MainMenuConfig      `json:"main_menu"`
	Integration1C Integration1CConfig `json:"integration_1c"`
	MAX           MAXConfig           `json:"max"`
}

type HTTPConfig struct {
	Addr           string `json:"addr"`
	ReadTimeout    string `json:"read_timeout"`
	WriteTimeout   string `json:"write_timeout"`
	IdleTimeout    string `json:"idle_timeout"`
	RequestTimeout string `json:"request_timeout"`
}

type DatabaseConfig struct {
	Primary     string            `json:"primary"`
	Secondaries map[string]string `json:"secondaries"`
}

type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAgeSeconds    int      `json:"max_age_seconds"`
}

type SessionConfig struct {
	Enabled        bool        `json:"enabled"`
	CookieName     string      `json:"cookie_name"`
	CookieDomain   string      `json:"cookie_domain"`
	Secure         bool        `json:"secure"`
	SameSite       string      `json:"same_site"`
	MaxLifeTime    int64       `json:"max_life_time"`
	MaxIdleTime    int64       `json:"max_idle_time"`
	DestroyAllTime string      `json:"destroy_all_time"`
	Redis          RedisConfig `json:"redis"`
}

type RedisConfig struct {
	Connect   string `json:"connect"`
	Namespace string `json:"namespace"`
}

type MainMenuConfig struct {
	CacheEnabled bool   `json:"cache_enabled"`
	CacheTTL     string `json:"cache_ttl"`
}

type MAXConfig struct {
	BotToken                      string `json:"bot_token"`
	HTTPAddr                      string `json:"http_addr"`
	WebhookSecret                 string `json:"webhook_secret"`
	WebhookURL                    string `json:"webhook_url"`
	MiniAppURL                    string `json:"mini_app_url"`
	InitDataMaxAge                string `json:"init_data_max_age"`
	SenderPollInterval            string `json:"sender_poll_interval"`
	SenderNotifyReconnectInterval string `json:"sender_notify_reconnect_interval"`
	SenderLockTimeout             string `json:"sender_lock_timeout"`
	SenderRetryBaseDelay          string `json:"sender_retry_base_delay"`
	SenderRetryMaxDelay           string `json:"sender_retry_max_delay"`
	SenderMaxAttempts             int    `json:"sender_max_attempts"`
}

type Integration1CConfig struct {
	URL        string                    `json:"url"`
	Timeout    string                    `json:"timeout"`
	MaxRetries int                       `json:"max_retries"`
	RetryDelay string                    `json:"retry_delay"`
	Worker     Integration1CWorkerConfig `json:"worker"`
}

type Integration1CWorkerConfig struct {
	Enabled                 bool   `json:"enabled"`
	WorkerID                string `json:"worker_id"`
	Concurrency             int    `json:"concurrency"`
	PollInterval            string `json:"poll_interval"`
	NotifyReconnectInterval string `json:"notify_reconnect_interval"`
	RequeueInterval         string `json:"requeue_interval"`
	LockTimeout             string `json:"lock_timeout"`
	JobTimeout              string `json:"job_timeout"`
	RetryBaseDelay          string `json:"retry_base_delay"`
	RetryMaxDelay           string `json:"retry_max_delay"`
	ResultPollInterval      string `json:"result_poll_interval"`
}

type DebugConfig struct {
	LogConfig  bool   `json:"log_config"`
	LogLevel   string `json:"log_level"`
	LogFormat  string `json:"log_format"`
	SQLQueries bool   `json:"sql_queries"`
}

func Load(path string) (Config, error) {
	if path == "" {
		path = DefaultConfigPath
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := defaultConfig()
	if err := json.Unmarshal(body, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func defaultConfig() Config {
	return Config{
		HTTP: HTTPConfig{
			Addr:           "127.0.0.1:8080",
			ReadTimeout:    "15s",
			WriteTimeout:   "15s",
			IdleTimeout:    "60s",
			RequestTimeout: "30s",
		},
		CORS: CORSConfig{
			AllowedOrigins: []string{"http://localhost:3000"},
			AllowedMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPatch,
				http.MethodPut,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowedHeaders: []string{
				"Content-Type",
				"Authorization",
				webapp.DefaultQueryIDHeader,
			},
			ExposedHeaders: []string{
				webapp.DefaultQueryIDHeader,
			},
			AllowCredentials: true,
			MaxAgeSeconds:    86400,
		},
		Debug: DebugConfig{
			LogLevel:  "info",
			LogFormat: "text",
		},
		Session: SessionConfig{
			CookieName:     "_s",
			SameSite:       "lax",
			MaxLifeTime:    86400,
			MaxIdleTime:    3600,
			DestroyAllTime: "03:00",
			Redis: RedisConfig{
				Connect:   "127.0.0.1:6379",
				Namespace: "supplier-test",
			},
		},
		MainMenu: MainMenuConfig{
			CacheEnabled: true,
			CacheTTL:     "15m",
		},
		MAX: MAXConfig{
			HTTPAddr:                      "127.0.0.1:59001",
			InitDataMaxAge:                "1h",
			SenderPollInterval:            "2s",
			SenderNotifyReconnectInterval: "5s",
			SenderLockTimeout:             "1m",
			SenderRetryBaseDelay:          "5s",
			SenderRetryMaxDelay:           "5m",
			SenderMaxAttempts:             5,
		},
		Integration1C: Integration1CConfig{
			Timeout:    "15s",
			MaxRetries: 2,
			RetryDelay: "250ms",
			Worker: Integration1CWorkerConfig{
				Concurrency:             2,
				PollInterval:            "2s",
				NotifyReconnectInterval: "5s",
				RequeueInterval:         "1m",
				LockTimeout:             "15m",
				JobTimeout:              "5m",
				RetryBaseDelay:          "5s",
				RetryMaxDelay:           "5m",
				ResultPollInterval:      "2s",
			},
		},
	}
}

func (c Config) Validate() error {
	if c.HTTP.Addr == "" {
		return fmt.Errorf("http.addr is required")
	}
	if c.Database.Primary == "" {
		return fmt.Errorf("database.primary is required")
	}
	if _, err := c.HTTPReadTimeout(); err != nil {
		return err
	}
	if _, err := c.HTTPWriteTimeout(); err != nil {
		return err
	}
	if _, err := c.HTTPIdleTimeout(); err != nil {
		return err
	}
	if _, err := c.RequestTimeout(); err != nil {
		return err
	}
	if c.MainMenu.CacheEnabled {
		cacheTTL, err := c.MainMenu.CacheTTLDuration()
		if err != nil {
			return err
		}
		if cacheTTL <= 0 {
			return fmt.Errorf("main_menu.cache_ttl should be positive")
		}
	}
	if c.MAX.InitDataMaxAge != "" {
		if _, err := c.MAX.InitDataMaxAgeDuration(); err != nil {
			return err
		}
	}
	maxDurations := []struct {
		name  string
		value string
	}{
		{"max.sender_poll_interval", c.MAX.SenderPollInterval},
		{"max.sender_notify_reconnect_interval", c.MAX.SenderNotifyReconnectInterval},
		{"max.sender_lock_timeout", c.MAX.SenderLockTimeout},
		{"max.sender_retry_base_delay", c.MAX.SenderRetryBaseDelay},
		{"max.sender_retry_max_delay", c.MAX.SenderRetryMaxDelay},
	}
	for _, item := range maxDurations {
		duration, err := parseDuration(item.name, item.value)
		if err != nil {
			return err
		}
		if duration <= 0 {
			return fmt.Errorf("%s should be positive", item.name)
		}
	}
	if c.MAX.SenderMaxAttempts <= 0 {
		return fmt.Errorf("max.sender_max_attempts should be positive")
	}
	retryBase, _ := c.MAX.SenderRetryBaseDelayDuration()
	retryMax, _ := c.MAX.SenderRetryMaxDelayDuration()
	if retryMax < retryBase {
		return fmt.Errorf("max.sender_retry_max_delay should be >= max.sender_retry_base_delay")
	}
	if c.Integration1C.URL != "" {
		timeout, err := c.Integration1C.TimeoutDuration()
		if err != nil {
			return err
		}
		if timeout <= 0 {
			return fmt.Errorf("integration_1c.timeout should be positive")
		}
		if c.Integration1C.MaxRetries < 0 {
			return fmt.Errorf("integration_1c.max_retries should not be negative")
		}
		retryDelay, err := c.Integration1C.RetryDelayDuration()
		if err != nil {
			return err
		}
		if retryDelay < 0 {
			return fmt.Errorf("integration_1c.retry_delay should not be negative")
		}
	}
	if c.Integration1C.Worker.Enabled {
		if c.Integration1C.URL == "" {
			return fmt.Errorf("integration_1c.url is required when worker is enabled")
		}
		if c.Integration1C.Worker.Concurrency <= 0 {
			return fmt.Errorf("integration_1c.worker.concurrency should be positive")
		}
		workerDurations := []struct {
			name  string
			value string
		}{
			{"integration_1c.worker.poll_interval", c.Integration1C.Worker.PollInterval},
			{"integration_1c.worker.notify_reconnect_interval", c.Integration1C.Worker.NotifyReconnectInterval},
			{"integration_1c.worker.requeue_interval", c.Integration1C.Worker.RequeueInterval},
			{"integration_1c.worker.lock_timeout", c.Integration1C.Worker.LockTimeout},
			{"integration_1c.worker.job_timeout", c.Integration1C.Worker.JobTimeout},
			{"integration_1c.worker.retry_base_delay", c.Integration1C.Worker.RetryBaseDelay},
			{"integration_1c.worker.retry_max_delay", c.Integration1C.Worker.RetryMaxDelay},
			{"integration_1c.worker.result_poll_interval", c.Integration1C.Worker.ResultPollInterval},
		}
		for _, item := range workerDurations {
			duration, err := parseDuration(item.name, item.value)
			if err != nil {
				return err
			}
			if duration <= 0 {
				return fmt.Errorf("%s should be positive", item.name)
			}
		}
		lockTimeout, _ := c.Integration1C.Worker.LockTimeoutDuration()
		jobTimeout, _ := c.Integration1C.Worker.JobTimeoutDuration()
		if lockTimeout <= jobTimeout {
			return fmt.Errorf("integration_1c.worker.lock_timeout should be greater than job_timeout")
		}
		retryBaseDelay, _ := c.Integration1C.Worker.RetryBaseDelayDuration()
		retryMaxDelay, _ := c.Integration1C.Worker.RetryMaxDelayDuration()
		if retryMaxDelay < retryBaseDelay {
			return fmt.Errorf("integration_1c.worker.retry_max_delay should be >= retry_base_delay")
		}
	}
	return nil
}

func (c DatabaseConfig) GetPrimary() string {
	return c.Primary
}

func (c DatabaseConfig) GetSecondaries() map[string]string {
	return c.Secondaries
}

func (c Config) HTTPReadTimeout() (time.Duration, error) {
	return parseDuration("http.read_timeout", c.HTTP.ReadTimeout)
}

func (c Config) HTTPWriteTimeout() (time.Duration, error) {
	return parseDuration("http.write_timeout", c.HTTP.WriteTimeout)
}

func (c Config) HTTPIdleTimeout() (time.Duration, error) {
	return parseDuration("http.idle_timeout", c.HTTP.IdleTimeout)
}

func (c Config) RequestTimeout() (time.Duration, error) {
	return parseDuration("http.request_timeout", c.HTTP.RequestTimeout)
}

func (c MainMenuConfig) CacheTTLDuration() (time.Duration, error) {
	return parseDuration("main_menu.cache_ttl", c.CacheTTL)
}

func (c Integration1CConfig) TimeoutDuration() (time.Duration, error) {
	return parseDuration("integration_1c.timeout", c.Timeout)
}

func (c Integration1CConfig) RetryDelayDuration() (time.Duration, error) {
	return parseDuration("integration_1c.retry_delay", c.RetryDelay)
}

func (c Integration1CWorkerConfig) PollIntervalDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.poll_interval", c.PollInterval)
}

func (c Integration1CWorkerConfig) NotifyReconnectIntervalDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.notify_reconnect_interval", c.NotifyReconnectInterval)
}

func (c Integration1CWorkerConfig) RequeueIntervalDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.requeue_interval", c.RequeueInterval)
}

func (c Integration1CWorkerConfig) LockTimeoutDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.lock_timeout", c.LockTimeout)
}

func (c Integration1CWorkerConfig) JobTimeoutDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.job_timeout", c.JobTimeout)
}

func (c Integration1CWorkerConfig) RetryBaseDelayDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.retry_base_delay", c.RetryBaseDelay)
}

func (c Integration1CWorkerConfig) RetryMaxDelayDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.retry_max_delay", c.RetryMaxDelay)
}

func (c Integration1CWorkerConfig) ResultPollIntervalDuration() (time.Duration, error) {
	return parseDuration("integration_1c.worker.result_poll_interval", c.ResultPollInterval)
}

func (c CORSConfig) WebappConfig() webapp.CORSConfig {
	return webapp.CORSConfig{
		AllowedOrigins:   c.AllowedOrigins,
		AllowedMethods:   c.AllowedMethods,
		AllowedHeaders:   c.AllowedHeaders,
		ExposedHeaders:   c.ExposedHeaders,
		AllowCredentials: c.AllowCredentials,
		MaxAgeSeconds:    c.MaxAgeSeconds,
	}
}

func (c SessionConfig) WebappConfig() websession.Config {
	return websession.Config{
		CookieName:     c.CookieName,
		CookieDomain:   c.CookieDomain,
		Secure:         c.Secure,
		HTTPOnly:       true,
		SameSite:       parseSameSite(c.SameSite),
		MaxLifeTime:    c.MaxLifeTime,
		MaxIdleTime:    c.MaxIdleTime,
		DestroyAllTime: c.DestroyAllTime,
	}
}

func (c RedisConfig) WebappConfig() websession.RedisConfig {
	return websession.RedisConfig{
		Connect:   c.Connect,
		Namespace: c.Namespace,
	}
}

func parseDuration(name string, value string) (time.Duration, error) {
	if value == "" {
		return 0, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}

	return duration, nil
}

func parseSameSite(value string) http.SameSite {
	switch value {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "lax", "":
		return http.SameSiteLaxMode
	default:
		return http.SameSiteLaxMode
	}
}

func (c MAXConfig) InitDataMaxAgeDuration() (time.Duration, error) {
	if c.InitDataMaxAge == "" {
		return 24 * time.Hour, nil
	}
	return parseDuration("max.init_data_max_age", c.InitDataMaxAge)
}

func (c MAXConfig) SenderPollIntervalDuration() (time.Duration, error) {
	return parseDuration("max.sender_poll_interval", c.SenderPollInterval)
}

func (c MAXConfig) SenderNotifyReconnectIntervalDuration() (time.Duration, error) {
	return parseDuration("max.sender_notify_reconnect_interval", c.SenderNotifyReconnectInterval)
}

func (c MAXConfig) SenderLockTimeoutDuration() (time.Duration, error) {
	return parseDuration("max.sender_lock_timeout", c.SenderLockTimeout)
}

func (c MAXConfig) SenderRetryBaseDelayDuration() (time.Duration, error) {
	return parseDuration("max.sender_retry_base_delay", c.SenderRetryBaseDelay)
}

func (c MAXConfig) SenderRetryMaxDelayDuration() (time.Duration, error) {
	return parseDuration("max.sender_retry_max_delay", c.SenderRetryMaxDelay)
}
