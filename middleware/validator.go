package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/LerianStudio/lib-commons/commons"
	"github.com/LerianStudio/lib-commons/commons/log"
	"github.com/LerianStudio/lib-commons/commons/zap"
	cn "github.com/LerianStudio/lib-license-go/constant"
	libErr "github.com/LerianStudio/lib-license-go/error"
	"github.com/LerianStudio/lib-license-go/model"
	"github.com/LerianStudio/lib-license-go/util"
	"github.com/dgraph-io/ristretto"
)

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
	cfg          model.Config
	logger       log.Logger
	cachedResult *model.ValidationResult
}

// NewLicenseClient creates a new license validator with the given config and logger.
// If logger is nil, defaults to a standard zap logger.
// The validator includes fingerprint generation, caching, and background validation.
func NewLicenseClient(
	applicationName string,
	licenseKey string,
	midazOrganizationID string,
	pluginEnvironment string,
	logger *log.Logger) *LicenseClient {
	var l log.Logger

	if logger != nil {
		l = *logger
	} else {
		l = zap.InitializeLogger()
	}

	cfg := &model.Config{
		ApplicationName:   applicationName,
		LicenseKey:        licenseKey,
		OrganizationID:    midazOrganizationID,
		PluginEnvironment: pluginEnvironment,
	}

	if err := util.ValidateEnvVariables(cfg, l); err != nil {
		l.Errorf("Invalid environment variables - error: %s", err.Error())

		return nil
	}

	cache, _ := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M)
		MaxCost:     1 << 20, // maximum cost of cache (1MB)
		BufferItems: 64,      // number of keys per Get buffer
	})

	fp := applicationName + ":"

	if midazOrganizationID != "" {
		fp = fp + commons.HashSHA256(licenseKey+":"+midazOrganizationID)
	} else {
		fp = fp + commons.HashSHA256(licenseKey)
	}

	cfg.Fingerprint = fp

	return &LicenseClient{
		cfg:   *cfg,
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
func (v *LicenseClient) Validate(ctx context.Context) (model.ValidationResult, error) {
	// First check cache
	if val, found := v.cache.Get(v.cfg.Fingerprint); found {
		if r, ok := val.(model.ValidationResult); ok {
			v.logger.Infof("License cached [expires: %d days | grace: %t]", r.ExpiryDaysLeft, r.ActiveGracePeriod)
			return r, nil
		}
	}

	// Not in cache, so call the license backend
	res, err := v.callBackend(ctx)
	if err != nil {
		// Custom error handling by status code
		if apiErr, ok := err.(*libErr.ApiError); ok {
			if apiErr.StatusCode >= 500 && apiErr.StatusCode < 600 {
				v.logger.Warnf("License server error (5xx) detected, treating as valid - error: %s", apiErr.Error())

				if v.cachedResult != nil {
					return *v.cachedResult, nil
				}

				return model.ValidationResult{
					Valid:             true,
					ExpiryDaysLeft:    cn.FallbackExpiryDaysLeft,
					ActiveGracePeriod: true,
				}, nil
			}

			if apiErr.StatusCode >= 400 && apiErr.StatusCode < 500 {
				v.logger.Errorf("Exiting: license validation failed with client error: %s", apiErr.Error())
				v.ShutdownBackgroundRefresh()
				panic("license validation failed with client error")
			}
		}

		// Connection errors should use cached result if available
		if libErr.IsConnectionError(err) && v.cachedResult != nil {
			v.logger.Warnf("Using cached license validation due to connection error - error: %s", err.Error())
			return *v.cachedResult, nil
		}

		return model.ValidationResult{}, fmt.Errorf("failed to validate license: %w", err)
	}

	// 200 OK: check license validity
	if !res.Valid && !res.ActiveGracePeriod {
		v.logger.Errorf("Exiting: license is not valid and no grace period is active!")
		v.ShutdownBackgroundRefresh()
		panic("license is not valid and no grace period is active!")
	}

	// Cache result (using fixed TTL for security)
	const cacheTTLHours = 24 // One day maximum, hardcoded for security
	cacheTTL := time.Duration(cacheTTLHours) * time.Hour
	v.cache.SetWithTTL(v.cfg.Fingerprint, res, 1, cacheTTL)

	// Store last successful result for fallback
	resultCopy := res
	v.cachedResult = &resultCopy

	return res, nil
}

// callBackend makes an API call to validate the license.
func (v *LicenseClient) callBackend(ctx context.Context) (model.ValidationResult, error) {
	if v.cfg.PluginEnvironment == "" {
		return model.ValidationResult{}, errors.New("PLUGIN_ENVIRONMENT not set")
	}
	url := fmt.Sprintf("https://np0e73vyt5.execute-api.us-east-2.amazonaws.com/%s/licenses/validate", v.cfg.PluginEnvironment)

	reqBody := map[string]string{
		"licenseKey":  v.cfg.LicenseKey,
		"fingerprint": v.cfg.Fingerprint,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.cli.Do(req)
	if err != nil {
		v.logger.Warnf("License validation request failed - error: %s", err.Error())
		return model.ValidationResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResp model.ErrorResponse

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		_ = json.Unmarshal(bodyBytes, &errorResp)

		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			v.logger.Debugf("Server error during license validation - status: %d, code: %s, message: %s",
				resp.StatusCode, errorResp.Code, errorResp.Message)
			return model.ValidationResult{}, &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("server error: %d", resp.StatusCode)}
		}
		// 4xx error: log and propagate
		if resp.StatusCode >= 400 && resp.StatusCode < 500 {
			v.logger.Debugf("Client error during license validation - status: %d, code: %s, message: %s",
				resp.StatusCode, errorResp.Code, errorResp.Message)
			return model.ValidationResult{}, &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("client error: %d", resp.StatusCode)}
		}

		v.logger.Debugf("Unexpected error during license validation - status: %d, code: %s, message: %s",
			resp.StatusCode, errorResp.Code, errorResp.Message)
		return model.ValidationResult{}, &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("unexpected error: %d", resp.StatusCode)}
	}

	var out model.ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	v.logger.Infof("License valid [expires: %d days | grace: %t]", out.ExpiryDaysLeft, out.ActiveGracePeriod)
	return out, nil
}

// StartBackgroundRefresh runs a ticker to refresh license weekly.
//
// Note: os.Exit(1) in Validate will terminate the process even if called from this goroutine.
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
}

// attemptValidationWithRetry tries to validate the license with exponential backoff
//
// Note: os.Exit(1) in Validate will terminate the process even if called from this goroutine.
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
			v.logger.Errorf("Background license validation failed after retries - error: %s", err.Error())
			return
		}

		// Exponential backoff: 1s, 2s, 4s
		backoffDuration := time.Duration(1<<uint(retryCount)) * time.Second
		v.logger.Warnf("Retrying license validation - attempt: %d, backoff_seconds: %.2f", retryCount, backoffDuration.Seconds())

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoffDuration):
			// Continue to next attempt
		}
	}
}
