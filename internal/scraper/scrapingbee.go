package scraper

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ScrapingBeeClient wraps HTTP requests through ScrapingBee's API
// to bypass bot protection like Kasada used by REA.
type ScrapingBeeClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// NewScrapingBeeClient creates a new ScrapingBee client
func NewScrapingBeeClient(apiKey string) *ScrapingBeeClient {
	return &ScrapingBeeClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 180 * time.Second, // ScrapingBee stealth mode can take up to 3 minutes
		},
		baseURL: "https://app.scrapingbee.com/api/v1/",
	}
}

// ScrapingBeeOptions configures the ScrapingBee request
type ScrapingBeeOptions struct {
	// RenderJS enables JavaScript rendering (needed for dynamic content)
	RenderJS bool
	// Premium uses premium proxies (residential IPs, better for anti-bot)
	Premium bool
	// Stealth uses stealth proxies (best for advanced anti-bot like Kasada)
	// Note: Uses 75 credits per request instead of 25
	Stealth bool
	// Country sets the proxy country (e.g., "au" for Australia)
	Country string
	// WaitForSelector waits for a CSS selector before returning
	WaitForSelector string
	// Wait adds a fixed delay in milliseconds after page load
	Wait int
	// BlockResources blocks images, stylesheets, etc. to speed up requests
	BlockResources bool
	// ReturnPageSource returns the page source instead of rendered HTML
	ReturnPageSource bool
}

// DefaultREAOptions returns options optimized for scraping REA
// Uses stealth proxy mode which is designed for sites with advanced bot protection
func DefaultREAOptions() ScrapingBeeOptions {
	return ScrapingBeeOptions{
		RenderJS:        true,                    // REA uses JavaScript for content
		Premium:         false,                   // Don't use premium when using stealth
		Stealth:         true,                    // Use stealth proxy for Kasada bypass
		Country:         "au",                    // Australian proxies for better success
		WaitForSelector: "a[href*='/property-']", // Wait for property links to appear
		Wait:            5000,                    // Additional 5 second wait after element found
		BlockResources:  false,                   // Don't block resources, we need full page
	}
}

// Fetch retrieves a URL through ScrapingBee
func (c *ScrapingBeeClient) Fetch(ctx context.Context, targetURL string, opts ScrapingBeeOptions) ([]byte, error) {
	// Build the ScrapingBee API URL with parameters
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("url", targetURL)

	if opts.RenderJS {
		params.Set("render_js", "true")
	}
	if opts.Stealth {
		// Stealth proxy is the most advanced option for anti-bot bypass
		// It uses real browser fingerprints and advanced evasion techniques
		params.Set("stealth_proxy", "true")
	} else if opts.Premium {
		params.Set("premium_proxy", "true")
	}
	if opts.Country != "" {
		params.Set("country_code", opts.Country)
	}
	if opts.WaitForSelector != "" {
		params.Set("wait_for", opts.WaitForSelector)
	}
	if opts.Wait > 0 {
		params.Set("wait", fmt.Sprintf("%d", opts.Wait))
	}
	if opts.BlockResources {
		params.Set("block_resources", "true")
	}
	if opts.ReturnPageSource {
		params.Set("return_page_source", "true")
	}

	apiURL := c.baseURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	// Check for ScrapingBee errors
	if resp.StatusCode != http.StatusOK {
		// ScrapingBee returns error details in the response body
		return nil, fmt.Errorf("ScrapingBee error (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// Check remaining credits from headers
	if credits := resp.Header.Get("Spb-Cost"); credits != "" {
		// Log credit usage for monitoring (can be parsed if needed)
		// The "Spb-Cost" header shows how many credits this request used
	}

	return body, nil
}

// FetchHTML is a convenience method that returns the response as a string
func (c *ScrapingBeeClient) FetchHTML(ctx context.Context, targetURL string, opts ScrapingBeeOptions) (string, error) {
	body, err := c.Fetch(ctx, targetURL, opts)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
