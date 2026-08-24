package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultReceiptAPIClientTimeout = 25 * time.Second

	// Error bodies from external APIs can be unexpectedly large HTML pages,
	// proxy responses, or diagnostic payloads. Keep only a small preview for
	// debugging and never load/log the whole body on non-2xx responses.
	maxReceiptAPIErrorBodySize = 4 * 1024
)

var (
	ErrReceiptAPIClientNil   = errors.New("receipt api client is nil")
	ErrReceiptAPIURLRequired = errors.New("receipt api url is required")
)

type ReceiptAPIClient struct {
	httpClient *http.Client
}

// NewReceiptAPIClient creates a reusable HTTP client wrapper.
//
// Reusing http.Client is important because Go keeps connection pools inside it.
// Creating a new client per request would prevent connection reuse and increase
// latency under load.
func NewReceiptAPIClient(httpClient *http.Client) *ReceiptAPIClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultReceiptAPIClientTimeout,
		}
	}

	return &ReceiptAPIClient{
		httpClient: httpClient,
	}
}

// Fetch requests receipt data from the public tax API.
//
// Context is part of the method contract because this call can be triggered by
// user requests, background jobs, or shutdown flows. The caller should be able
// to cancel the HTTP request instead of waiting for the external API forever.
func (c *ReceiptAPIClient) Fetch(ctx context.Context, apiURL string) (*ReceiptResponse, error) {
	if c == nil {
		return nil, ErrReceiptAPIClientNil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return nil, ErrReceiptAPIURLRequired
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create receipt API request: %w", err)
	}

	setReceiptRequestHeaders(req)

	httpClient := c.httpClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultReceiptAPIClientTimeout,
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send receipt API request: %w", err)
	}
	defer resp.Body.Close()

	if err := validateReceiptAPIStatus(resp); err != nil {
		return nil, err
	}

	var apiResponse ReceiptResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, fmt.Errorf("decode receipt API JSON response: %w", err)
	}

	return &apiResponse, nil
}

func validateReceiptAPIStatus(resp *http.Response) error {
	if resp == nil {
		return errors.New("receipt API response is nil")
	}

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	bodyPreview, _ := io.ReadAll(io.LimitReader(resp.Body, maxReceiptAPIErrorBodySize))
	if len(bodyPreview) == 0 {
		return fmt.Errorf("receipt API returned non-success status: %s", resp.Status)
	}

	return fmt.Errorf(
		"receipt API returned non-success status: %s, body_preview=%q",
		resp.Status,
		string(bodyPreview),
	)
}

// setReceiptRequestHeaders mirrors the browser-like headers expected by the tax API.
//
// The endpoint is public, but it is primarily used by the official web page.
// These headers reduce the chance of backend requests behaving differently from
// requests made through the tax cabinet UI.
func setReceiptRequestHeaders(req *http.Request) {
	if req == nil {
		return
	}

	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/124.0 Safari/537.36",
	)

	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Referer", "https://cabinet.tax.gov.ua/cashregs/check")
	req.Header.Set("Origin", "https://cabinet.tax.gov.ua")
}
