package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LerianStudio/lib-commons/commons"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/zap"
	"github.com/dgraph-io/ristretto"
	"github.com/google/uuid"
)

type Config struct {
	ApplicationName string
	LicenseKey      string
	OrganizationID  string
	APIGatewayURL   string

	fingerprint string
}

// ValidationResult contains the data returned by license validation.
type ValidationResult struct {
	Valid          bool `json:"valid"`
	ExpiryDaysLeft int  `json:"expiryDaysLeft,omitempty"`
}

// backgroundRefreshConfig holds configuration for background refresh
type backgroundRefreshConfig struct {
	refreshInterval      time.Duration
	started              bool
	mu                   sync.Mutex
	cancel               context.CancelFunc
	lastAttemptedRefresh time.Time
}

// LicenseClient handles license validation with caching and background refresh.
type LicenseClient struct {
	cli          *http.Client
	cache        *ristretto.Cache
	bgConfig     *backgroundRefreshConfig
	cfg          Config
	logger       log.Logger
	cachedResult *ValidationResult
}

// NewLicenseClient creates a new license validator with the given config and logger.
// If logger is nil, defaults to a standard zap logger.
// The validator includes fingerprint generation, caching, and background validation.
func NewLicenseClient(cfg Config, logger *log.Logger) *LicenseClient {
	var l log.Logger

	if logger != nil {
		l = *logger
	} else {
		l = zap.InitializeLogger()
	}

	if err := validateEnvVariables(&cfg, l); err != nil {
		l.Error("Invalid environment variables", "error", err.Error())

		return nil
	}

	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 20, // maximum cost of cache (1MB)
		BufferItems: 64,      // number of keys per Get buffer
	})

	var fp string

	if cfg.fingerprint == "" {
		fp = uuid.NewString() + "_" + cfg.ApplicationName
	} else {
		fp = cfg.fingerprint
	}

	if org := cfg.OrganizationID; org != "" {
		fp = org + "_" + fp
	}

	cfg.fingerprint = fp

	return &LicenseClient{
		cfg:   cfg,
		cache: cache,
		cli: &http.Client{
			Timeout: 5 * time.Second,
		},
		bgConfig: &backgroundRefreshConfig{
			refreshInterval: 7 * 24 * time.Hour,
		},
		logger: l,
	}
}

// Validate checks if the license is valid. Results are cached.
func (v *LicenseClient) Validate(ctx context.Context) (ValidationResult, error) {
	// First check cache
	if val, found := v.cache.Get("license"); found {
		if r, ok := val.(ValidationResult); ok {
			v.logger.Info("Using cached license validation", "expires_in_days", r.ExpiryDaysLeft)
			return r, nil
		}
	}

	// Not in cache, so call the license backend
	res, err := v.callBackend(ctx)
	if err != nil {
		// Connection errors should use cached result if available
		if isConnectionError(err) && v.cachedResult != nil {
			v.logger.Warn("Using cached license validation due to connection error", "error", err.Error())
			return *v.cachedResult, nil
		}
		return ValidationResult{}, fmt.Errorf("failed to validate license: %w", err)
	}

	// Cache result (using fixed TTL for security)
	const cacheTTLHours = 24 // One day maximum, hardcoded for security
	cacheTTL := time.Duration(cacheTTLHours) * time.Hour
	v.cache.SetWithTTL("license", res, 1, cacheTTL)

	// Store last successful result for fallback
	resultCopy := res
	v.cachedResult = &resultCopy

	return res, nil
}

// callBackend makes an API call to validate the license.
func (v *LicenseClient) callBackend(ctx context.Context) (ValidationResult, error) {
	if v.cfg.APIGatewayURL == "" {
		return ValidationResult{}, errors.New("LERIAN_API_GATEWAY_URL not set")
	}
	url := fmt.Sprintf("%s/validate-token", v.cfg.APIGatewayURL)

	reqBody := map[string]string{
		"licenseKey":  v.cfg.LicenseKey,
		"fingerprint": v.cfg.fingerprint,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return ValidationResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return ValidationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.cli.Do(req)
	if err != nil {
		v.logger.Warn("License validation request failed", "error", err.Error())
		return ValidationResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ValidationResult{}, fmt.Errorf("server returned non-200 status: %d", resp.StatusCode)
	}

	var out ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ValidationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	v.logger.Info("License validated successfully", "expires_in_days", out.ExpiryDaysLeft)
	return out, nil
}

// StartBackgroundRefresh runs a ticker to refresh license weekly.
func (v *LicenseClient) StartBackgroundRefresh(ctx context.Context) {
	v.bgConfig.mu.Lock()
	if v.bgConfig.started {
		v.bgConfig.mu.Unlock()
		return
	}
	v.bgConfig.started = true
	v.bgConfig.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	v.bgConfig.cancel = cancel

	go func() {
		// Initial validation with retry
		v.attemptValidationWithRetry(ctx)

		ticker := time.NewTicker(v.bgConfig.refreshInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				v.logger.Info("Background license validation stopped")
				return
			case <-ticker.C:
				v.attemptValidationWithRetry(ctx)
			}
		}
	}()

	v.logger.Info("Started background license validation",
		"interval_hours", v.bgConfig.refreshInterval.Hours())
}

// attemptValidationWithRetry tries to validate the license with exponential backoff
func (v *LicenseClient) attemptValidationWithRetry(ctx context.Context) {
	v.bgConfig.mu.Lock()
	v.bgConfig.lastAttemptedRefresh = time.Now()
	v.bgConfig.mu.Unlock()

	retryCount := 0
	maxRetries := 3

	for retryCount < maxRetries {
		_, err := v.Validate(ctx)
		if err == nil {
			return
		}

		retryCount++
		if retryCount >= maxRetries {
			v.logger.Error("Background license validation failed after retries", "error", err.Error())
			return
		}

		// Exponential backoff: 1s, 2s, 4s
		backoffDuration := time.Duration(1<<uint(retryCount)) * time.Second
		v.logger.Warn("Retrying license validation", "attempt", retryCount, "backoff_seconds", backoffDuration.Seconds())

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoffDuration):
			// Continue to next attempt
		}
	}
}

// isConnectionError checks if an error is likely related to network connectivity
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error interface implementations
	if netErr, ok := err.(net.Error); ok && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	// Check common error messages
	errText := strings.ToLower(err.Error())
	connectionErrorPatterns := []string{
		"connection refused",
		"no such host",
		"timeout",
		"connection reset",
		"eof",
		"broken pipe",
		"tls handshake",
		"i/o timeout",
		"network is unreachable",
	}

	for _, pattern := range connectionErrorPatterns {
		if strings.Contains(errText, pattern) {
			return true
		}
	}

	return false
}

func validateEnvVariables(cfg *Config, l log.Logger) error {
	if commons.IsNilOrEmpty(&cfg.ApplicationName) {
		err := "Missing application name environment variable"

		l.Error(err)

		return errors.New(err)
	}

	if commons.IsNilOrEmpty(&cfg.LicenseKey) {
		err := "Missing license key environment variable"

		l.Error(err)

		return errors.New(err)
	}

	if commons.IsNilOrEmpty(&cfg.OrganizationID) {
		err := "Missing organization ID environment variable"

		l.Error(err)

		return errors.New(err)
	}

	if commons.IsNilOrEmpty(&cfg.APIGatewayURL) {
		err := "Missing api gateway url environment variable"

		l.Error(err)

		return errors.New(err)
	}

	return nil
}
