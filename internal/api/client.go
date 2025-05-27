package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
}

// New creates a new API client
func New(cfg *config.ClientConfig, httpClient *http.Client, logger log.Logger) *Client {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: cfg.HTTPTimeout,
		}
	}

	return &Client{
		httpClient: httpClient,
		config:     cfg,
		logger:     logger,
	}
}

// SetHTTPClient allows overriding the HTTP client (useful for testing)
func (c *Client) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.httpClient = client
	}
}

// baseURL is used to store the license validation API URL
// By default, it is set to the fixed value and can only be changed via internal test helpers
var baseURL = cn.DefaultLicenseGatewayBaseURL

// ValidateLicense performs the license validation API call
func (c *Client) ValidateLicense(ctx context.Context) (model.ValidationResult, error) {
	if c.config.Environment == "" {
		return model.ValidationResult{}, fmt.Errorf("environment is not set")
	}

	url := fmt.Sprintf("%s/%s/licenses/validate", baseURL, c.config.Environment)
	
	reqBody := map[string]string{
		"licenseKey":  c.config.LicenseKey,
		"fingerprint": c.config.Fingerprint,
	}
	
	body, err := json.Marshal(reqBody)
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to create request: %w", err)
	}
	// Set required headers
	req.Header.Set("Content-Type", "application/json")
	
	// Add organization ID as API key in header
	if c.config.OrganizationID != "" {
		req.Header.Set("x-api-key", c.config.OrganizationID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Warnf("License validation request failed - error: %s", err.Error())
		return model.ValidationResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
	}

	var result model.ValidationResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return model.ValidationResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

// handleErrorResponse processes error responses from the API
func (c *Client) handleErrorResponse(resp *http.Response) (model.ValidationResult, error) {
	var errorResp model.ErrorResponse

	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	_ = json.Unmarshal(bodyBytes, &errorResp)

	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		c.logger.Debugf("Server error during license validation - status: %d, code: %s, message: %s",
			resp.StatusCode, errorResp.Code, errorResp.Message)
		return model.ValidationResult{}, &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("server error: %d", resp.StatusCode)}
	}
	
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		apiErr := &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("client error: %d", resp.StatusCode)}
		c.logger.Debugf("Client error during license validation - status: %d, code: %s, message: %s",
			resp.StatusCode, errorResp.Code, errorResp.Message)
		
		return model.ValidationResult{}, apiErr
	}

	c.logger.Debugf("Unexpected error during license validation - status: %d, code: %s, message: %s",
		resp.StatusCode, errorResp.Code, errorResp.Message)
	return model.ValidationResult{}, &libErr.ApiError{StatusCode: resp.StatusCode, Msg: fmt.Sprintf("unexpected error: %d", resp.StatusCode)}
}
