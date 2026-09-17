// Package config loads and validates runtime settings from the environment.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAddress         = ":8080"
	defaultDatabasePath    = "data/forum.db"
	defaultSessionDuration = 24 * time.Hour
	defaultCookieName      = "forum_session"
	defaultSecureCookie    = false
	defaultHTTPSEnabled    = false
)

// for providers authentication.
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	Enabled      bool
}

// Config contains every runtime value required to construct the application.
type Config struct {
	Address         string
	DatabasePath    string
	SessionDuration time.Duration
	CookieName      string
	SecureCookie    bool
	GitHub          OAuthProviderConfig
	Google          OAuthProviderConfig
	HTTPSEnabled    bool
	TLSCertFile     string
	TLSKeyFile      string
}

// Load applies defaults, reads FORUM_* overrides, and validates the result.
func Load() (Config, error) {
	cfg := Config{
		Address:         envOrDefault("FORUM_ADDRESS", defaultAddress),
		DatabasePath:    envOrDefault("FORUM_DATABASE_PATH", defaultDatabasePath),
		SessionDuration: defaultSessionDuration,
		CookieName:      envOrDefault("FORUM_COOKIE_NAME", defaultCookieName),
		SecureCookie:    defaultSecureCookie,
		HTTPSEnabled:    defaultHTTPSEnabled,
		TLSCertFile:     os.Getenv("FORUM_TLS_CERT_FILE"),
		TLSKeyFile:      os.Getenv("FORUM_TLS_KEY_FILE"),
	}
	cfg.GitHub = OAuthProviderConfig{
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GITHUB_REDIRECT_URL"),
	}

	cfg.Google = OAuthProviderConfig{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}

	durationValue := os.Getenv("FORUM_SESSION_DURATION")
	if durationValue != "" {
		duration, err := time.ParseDuration(durationValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid session duration: %w", err)
		}

		cfg.SessionDuration = duration
	}

	secureCookieValue := os.Getenv("FORUM_SECURE_COOKIE")
	if secureCookieValue != "" {
		secureCookie, err := strconv.ParseBool(secureCookieValue)
		if err != nil {
			return Config{}, fmt.Errorf("invalid secure cookie value: %w", err)
		}

		cfg.SecureCookie = secureCookie
	}

	httpsEnabledValue := os.Getenv("FORUM_HTTPS_ENABLED")
	if httpsEnabledValue != "" {
		httpsEnabled, err := strconv.ParseBool(httpsEnabledValue)
		if err != nil {
			return Config{}, fmt.Errorf(
				"invalid HTTPS enabled value: %w",
				err,
			)
		}

		cfg.HTTPSEnabled = httpsEnabled
	}

	if err := validate(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func validate(cfg *Config) error {
	if err := validateAddress(cfg.Address); err != nil {
		return err
	}

	if strings.TrimSpace(cfg.DatabasePath) == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if cfg.SessionDuration <= 0 {
		return fmt.Errorf("session duration must be greater than zero")
	}

	if err := validateCookieName(cfg.CookieName); err != nil {
		return err
	}
	if err := validateHTTPS(cfg); err != nil {
		return err
	}
	if err := validateOAuthProvider("github", &cfg.GitHub); err != nil {
		return err
	}

	if err := validateOAuthProvider("google", &cfg.Google); err != nil {
		return err
	}

	return nil
}

func validateAddress(address string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("address cannot be empty")
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid address port %q: %w", port, err)
	}

	if portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("address port must be between 1 and 65535")
	}

	return nil
}

func validateCookieName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("cookie name cannot be empty")
	}

	for _, character := range name {
		if character <= 32 ||
			character >= 127 ||
			character == '(' ||
			character == ')' ||
			character == '<' ||
			character == '>' ||
			character == '@' ||
			character == ',' ||
			character == ';' ||
			character == ':' ||
			character == '\\' ||
			character == '"' ||
			character == '/' ||
			character == '[' ||
			character == ']' ||
			character == '?' ||
			character == '=' ||
			character == '{' ||
			character == '}' {
			return fmt.Errorf("cookie name contains invalid character %q", character)
		}
	}

	return nil
}

func validateOAuthProvider(name string, cfg *OAuthProviderConfig) error {
	values := []string{
		cfg.ClientID,
		cfg.ClientSecret,
		cfg.RedirectURL,
	}

	set := 0

	for _, value := range values {
		if value != "" {
			set++
		}
	}

	if set == 0 {
		cfg.Enabled = false
		return nil
	}

	if set != len(values) {
		return fmt.Errorf("%s oauth configuration is incomplete", name)
	}

	u, err := url.Parse(cfg.RedirectURL)
	if err != nil {
		return fmt.Errorf("%s oauth redirect URL: %w", name, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s oauth redirect URL must use http or https", name)
	}

	if u.Host == "" {
		return fmt.Errorf("%s oauth redirect URL must include a host", name)
	}

	cfg.Enabled = true

	return nil
}

func validateHTTPS(cfg *Config) error {
	hasCertificate := strings.TrimSpace(cfg.TLSCertFile) != ""
	hasPrivateKey := strings.TrimSpace(cfg.TLSKeyFile) != ""

	if hasCertificate != hasPrivateKey {
		return fmt.Errorf(
			"TLS certificate and key paths must be configured together",
		)
	}

	if cfg.HTTPSEnabled && !hasCertificate {
		return fmt.Errorf(
			"HTTPS requires TLS certificate and key paths",
		)
	}

	return nil
}
