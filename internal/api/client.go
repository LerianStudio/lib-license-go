package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/LerianStudio/lib-commons/commons/log"
	cn "github.com/LerianStudio/lib-license-go/constant"
	libErr "github.com/LerianStudio/lib-license-go/error"
	"github.com/LerianStudio/lib-license-go/internal/config"
	"github.com/LerianStudio/lib-license-go/model"
)

// Client handles communication with the license API
type Client struct {
	httpClient *http.Client
	config     *config.ClientConfig
	logger     log.Logger
	// IsGlobal indicates if this client is operating in global plugin mode
	IsGlobal bool
}

// New creates a new API client
func New(cfg *config.ClientConfig, httpClient *http.Client, logger log.Logger) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: cfg.HTTPTimeout,
		}
	}

	// Check if there's only one organization ID and it's the global plugin value
	isGlobal := len(cfg.OrganizationIDs) == 1 && cfg.OrganizationIDs[0] == cn.GlobalPluginValue

	return &Client{
		httpClient: httpClient,
		config:     cfg,
		logger:     logger,
		IsGlobal:   isGlobal,
	}
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

// GetHTTPClient returns the current HTTP client
func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

// baseURL is used to store the license validation API URL
// It's initialized from LICENSE_URL environment variable if available,
// otherwise defaults to the predefined value
var baseURL = getBaseURLFromEnvOrDefault()

// getBaseURLFromEnvOrDefault returns the license URL based on IS_DEVELOPMENT
// If true, returns the dev URL, otherwise returns the production URL
func getBaseURLFromEnvOrDefault() string {
	isDev := os.Getenv(cn.EnvIsDevelopment)
	if isDev == "true" {
		return cn.DevLicenseGatewayBaseURL
	}

	return cn.ProdLicenseGatewayBaseURL
}

// ValidateLicense performs the license validation API call for a specific organization ID
func (c *Client) ValidateLicense(ctx context.Context) (model.ValidationResult, error) {
	// If no organization IDs are configured, return an error
	if len(c.config.OrganizationIDs) == 0 {
		return model.ValidationResult{}, fmt.Errorf("no organization IDs configured")
	}

	// Try to validate with each organization ID
	var lastErr error

	for _, orgID := range c.config.OrganizationIDs {
		result, err := c.validateForOrganization(ctx, orgID)
		if err == nil && (result.Valid || result.ActiveGracePeriod) {
			// If any organization has a valid license, return success
			return result, nil
		}

		lastErr = err
	}

	// If we reach here, no valid license was found
	return model.ValidationResult{}, lastErr
}

// validateForOrganization performs the license validation API call for a specific organization ID
func (c *Client) validateForOrganization(ctx context.Context, orgID string) (model.ValidationResult, error) {
	url := fmt.Sprintf("%s/licenses/validate", baseURL)

	// Request body with application name, organization ID, and license key
	reqBody := map[string]string{
		"resourceName":   c.config.AppName,
		"licenseKey":     c.config.LicenseKey,
		"organizationId": orgID,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	// Set required headers
	req.Header.Set("Content-Type", "application/json")

	// Add organization ID as API key in header
	req.Header.Set("x-api-key", c.config.LicenseKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warnf("License validation request failed - error: %s", err.Error())
		return model.ValidationResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := c.handleErrorResponse(resp)
		return model.ValidationResult{}, err
	}

	var result model.ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// handleErrorResponse processes non-200 HTTP responses.
// Returns an appropriate APIError based on the status code
func (c *Client) handleErrorResponse(resp *http.Response) error {
	var errorResp model.ErrorResponse

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	_ = json.Unmarshal(bodyBytes, &errorResp)

	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		c.logger.Debugf("Server error during license validation - status: %d, code: %s, message: %s",
			resp.StatusCode, errorResp.Code, errorResp.Message)

		return &libErr.APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("server error: %d", resp.StatusCode)}
	}

	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		apiErr := &libErr.APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("client error: %d", resp.StatusCode)}
		c.logger.Debugf("Client error during license validation - status: %d, code: %s, message: %s",
			resp.StatusCode, errorResp.Code, errorResp.Message)

		return apiErr
	}

	c.logger.Debugf("Unexpected error during license validation - status: %d, code: %s, message: %s",
		resp.StatusCode, errorResp.Code, errorResp.Message)

	return &libErr.APIError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("unexpected error: %d", resp.StatusCode)}
}
