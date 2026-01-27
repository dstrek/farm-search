package scraper

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"farm-search/internal/models"
)

// BrowserScraper uses headless Chrome to scrape REA with stealth mode
type BrowserScraper struct {
	allocCtx    context.Context
	cancel      context.CancelFunc
	headless    bool
	cookies     []*network.CookieParam // Cookies to inject
	cookiesSet  bool                   // Track if cookies have been set
	userDataDir string                 // Path to Chrome user data directory for persistent sessions
}

// Cookie represents a browser cookie for JSON serialization
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path,omitempty"`
	Expires  float64 `json:"expires,omitempty"`
	HTTPOnly bool    `json:"httpOnly,omitempty"`
	Secure   bool    `json:"secure,omitempty"`
	SameSite string  `json:"sameSite,omitempty"`
}

// NewBrowserScraper creates a new browser-based scraper
func NewBrowserScraper(headless bool) *BrowserScraper {
	return &BrowserScraper{
		headless: headless,
	}
}

// SetUserDataDir sets the Chrome user data directory to use an existing browser profile
// This allows reusing an existing session where Kasada challenges have been solved
//
// Common locations:
// - macOS: ~/Library/Application Support/Google/Chrome
// - Linux: ~/.config/google-chrome or ~/.config/chromium
// - Windows: %LOCALAPPDATA%\Google\Chrome\User Data
func (s *BrowserScraper) SetUserDataDir(dir string) {
	s.userDataDir = dir
	log.Printf("Using Chrome user data directory: %s", dir)
}

// LoadCookiesFromFile loads cookies from a JSON file
// The file should contain an array of cookie objects with name, value, domain fields
// You can export cookies from your browser using extensions like "EditThisCookie" or "Cookie-Editor"
func (s *BrowserScraper) LoadCookiesFromFile(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read cookie file: %w", err)
	}

	var cookies []Cookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		return fmt.Errorf("failed to parse cookie file: %w", err)
	}

	s.cookies = make([]*network.CookieParam, 0, len(cookies))
	for _, c := range cookies {
		// Only include realestate.com.au cookies
		if !strings.Contains(c.Domain, "realestate.com.au") {
			continue
		}

		cookie := &network.CookieParam{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
		}
		if c.Path != "" {
			cookie.Path = c.Path
		}
		if c.Secure {
			cookie.Secure = true
		}
		if c.HTTPOnly {
			cookie.HTTPOnly = true
		}
		if c.SameSite != "" {
			switch strings.ToLower(c.SameSite) {
			case "strict":
				cookie.SameSite = network.CookieSameSiteStrict
			case "lax":
				cookie.SameSite = network.CookieSameSiteLax
			case "none":
				cookie.SameSite = network.CookieSameSiteNone
			}
		}
		s.cookies = append(s.cookies, cookie)
	}

	log.Printf("Loaded %d cookies for realestate.com.au", len(s.cookies))
	return nil
}

// SetCookies sets cookies directly (useful for passing from command line)
func (s *BrowserScraper) SetCookies(cookies []*network.CookieParam) {
	s.cookies = cookies
	log.Printf("Set %d cookies", len(s.cookies))
}

// Start initializes the browser with stealth mode settings
func (s *BrowserScraper) Start() error {
	// Build options based on headless mode
	var opts []chromedp.ExecAllocatorOption

	// If using a user data directory, start with minimal options to preserve the profile
	if s.userDataDir != "" {
		opts = []chromedp.ExecAllocatorOption{
			chromedp.NoFirstRun,
			chromedp.NoDefaultBrowserCheck,
			chromedp.UserDataDir(s.userDataDir),
			chromedp.Flag("disable-background-networking", false),
			chromedp.Flag("disable-extensions", false),
			chromedp.Flag("disable-sync", false),
			chromedp.Flag("disable-default-apps", false),
			chromedp.WindowSize(1920, 1080),
		}

		if s.headless {
			opts = append(opts, chromedp.Flag("headless", "new"))
		} else {
			opts = append(opts, chromedp.Flag("headless", false))
		}

		log.Printf("Starting browser with user profile from: %s", s.userDataDir)
	} else {
		// Standard stealth mode without user profile
		if s.headless {
			opts = append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", "new"),
				chromedp.Flag("disable-gpu", false),
			)
		} else {
			opts = append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", false),
			)
		}

		// Common options
		opts = append(opts,
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("disable-features", "IsolateOrigins,site-per-process,TranslateUI"),
			chromedp.Flag("disable-infobars", true),
			chromedp.Flag("disable-default-apps", true),
			chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
			chromedp.Flag("disable-breakpad", true),
			chromedp.Flag("disable-component-update", true),
			chromedp.Flag("disable-domain-reliability", true),
			chromedp.Flag("disable-hang-monitor", true),
			chromedp.Flag("disable-popup-blocking", true),
			chromedp.Flag("disable-prompt-on-repost", true),
			chromedp.Flag("disable-sync", true),
			chromedp.Flag("password-store", "basic"),
			chromedp.Flag("use-mock-keychain", true),
			chromedp.WindowSize(1920, 1080),
			chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36"),
		)
	}

	s.allocCtx, s.cancel = chromedp.NewExecAllocator(context.Background(), opts...)
	return nil
}

// Stop closes the browser
func (s *BrowserScraper) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// randomDelay returns a random duration between min and max milliseconds
func randomDelay(minMs, maxMs int) time.Duration {
	return time.Duration(minMs+rand.Intn(maxMs-minMs)) * time.Millisecond
}

// stealthScript returns JavaScript to make the browser appear more human
// This is injected after page load to patch detectable properties
func stealthScript() string {
	return `
		try {
			// Remove webdriver property
			Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
			delete Object.getPrototypeOf(navigator).webdriver;
			
			// Fix languages to Australian English
			Object.defineProperty(navigator, 'languages', {get: () => ['en-AU', 'en-US', 'en']});
			Object.defineProperty(navigator, 'language', {get: () => 'en-AU'});
			
			// Fix chrome object (present in real Chrome)
			if (!window.chrome) {
				window.chrome = {
					runtime: {
						onMessage: { addListener: function() {} },
						onConnect: { addListener: function() {} }
					},
					loadTimes: function() { return {}; },
					csi: function() { return {}; },
					app: { isInstalled: false }
				};
			}
			
			// Fix plugins array
			Object.defineProperty(navigator, 'plugins', {
				get: () => {
					const arr = [
						{ name: 'Chrome PDF Plugin', filename: 'internal-pdf-viewer', description: 'Portable Document Format' },
						{ name: 'Chrome PDF Viewer', filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai', description: '' },
						{ name: 'Native Client', filename: 'internal-nacl-plugin', description: '' }
					];
					arr.item = (i) => arr[i] || null;
					arr.namedItem = (name) => arr.find(p => p.name === name) || null;
					arr.refresh = () => {};
					return arr;
				}
			});
			
			// Fix permissions
			const originalQuery = window.navigator.permissions.query;
			window.navigator.permissions.query = (parameters) => (
				parameters.name === 'notifications' 
					? Promise.resolve({ state: Notification.permission }) 
					: originalQuery(parameters)
			);
			
			// Fix WebGL vendor/renderer  
			const getParameter = WebGLRenderingContext.prototype.getParameter;
			WebGLRenderingContext.prototype.getParameter = function(parameter) {
				if (parameter === 37445) return 'Intel Inc.';
				if (parameter === 37446) return 'Intel Iris OpenGL Engine';
				return getParameter.call(this, parameter);
			};
			
		} catch(e) {
			console.log('Stealth script error:', e);
		}
		true;
	`
}

// preloadStealthScript returns JavaScript to be injected BEFORE page load
// This uses Page.addScriptToEvaluateOnNewDocument to run before any page scripts
func preloadStealthScript() string {
	return `
		// This runs before any page JavaScript
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined,
			configurable: true
		});
		
		// Prevent detection via iframe contentWindow
		const originalContentWindow = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'contentWindow');
		Object.defineProperty(HTMLIFrameElement.prototype, 'contentWindow', {
			get: function() {
				const win = originalContentWindow.get.call(this);
				if (win) {
					try {
						Object.defineProperty(win.navigator, 'webdriver', {
							get: () => undefined,
							configurable: true
						});
					} catch(e) {}
				}
				return win;
			}
		});
		
		// Fix Function.prototype.toString to hide modifications
		const originalToString = Function.prototype.toString;
		Function.prototype.toString = function() {
			if (this === Function.prototype.toString) {
				return 'function toString() { [native code] }';
			}
			return originalToString.call(this);
		};
	`
}

// ScrapeListings scrapes property listings from REA using a real browser
func (s *BrowserScraper) ScrapeListings(ctx context.Context, region, propertyType string, maxPages int) ([]models.Property, error) {
	var allListings []models.Property
	seenIDs := make(map[string]bool)

	for page := 1; maxPages <= 0 || page <= maxPages; page++ {
		select {
		case <-ctx.Done():
			return allListings, ctx.Err()
		default:
		}

		log.Printf("Scraping REA page %d of %s/%s...", page, region, propertyType)

		listings, hasMore, err := s.scrapePage(ctx, region, propertyType, page)
		if err != nil {
			log.Printf("Error scraping page %d: %v", page, err)
			// If blocked, try waiting longer before giving up
			if strings.Contains(err.Error(), "blocked") {
				log.Printf("Detected bot protection, waiting 30 seconds before retry...")
				time.Sleep(30 * time.Second)
				listings, hasMore, err = s.scrapePage(ctx, region, propertyType, page)
				if err != nil {
					log.Printf("Still blocked after retry, stopping")
					break
				}
			} else {
				break
			}
		}

		// Deduplicate listings
		for _, l := range listings {
			if !seenIDs[l.ExternalID] {
				seenIDs[l.ExternalID] = true
				allListings = append(allListings, l)
			}
		}

		log.Printf("Found %d new listings on page %d (total: %d)", len(listings), page, len(allListings))

		if !hasMore || len(listings) == 0 {
			log.Printf("No more pages available")
			break
		}

		// Random delay between pages to appear more human (3-6 seconds)
		delay := randomDelay(3000, 6000)
		log.Printf("Waiting %v before next page...", delay)
		time.Sleep(delay)
	}

	return allListings, nil
}

func (s *BrowserScraper) scrapePage(ctx context.Context, region, propertyType string, pageNum int) ([]models.Property, bool, error) {
	// Build the search URL for rural/acreage properties with minimum 10 hectares (100,000 sqm)
	// Format: https://www.realestate.com.au/buy/property-land-acreage-rural-size-100000-in-nsw/list-1
	searchURL := fmt.Sprintf(
		"https://www.realestate.com.au/buy/property-land-acreage-rural-size-100000-in-%s/list-%d?activeSort=list-date",
		region, pageNum,
	)

	// Create a new browser context for each page
	taskCtx, cancel := chromedp.NewContext(s.allocCtx)
	defer cancel()

	// Set timeout
	taskCtx, cancel = context.WithTimeout(taskCtx, 60*time.Second)
	defer cancel()

	var html string
	var pageURL string

	// Step 1: Add preload script that runs BEFORE any page JavaScript
	// This is the key to avoiding detection - the script runs before Kasada's fingerprinting
	err := chromedp.Run(taskCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(preloadStealthScript()).Do(ctx)
			return err
		}),
	)
	if err != nil {
		log.Printf("Warning: failed to add preload script: %v", err)
	}

	// Step 2: Navigate to the actual page
	err = chromedp.Run(taskCtx,
		chromedp.Navigate(searchURL),
		chromedp.WaitReady("body"),
	)
	if err != nil {
		return nil, false, fmt.Errorf("navigation failed: %w", err)
	}

	// Step 3: Inject stealth script and wait
	err = chromedp.Run(taskCtx,
		chromedp.Evaluate(stealthScript(), nil),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return nil, false, fmt.Errorf("stealth injection failed: %w", err)
	}

	// Check if we're on a challenge page and wait for it to resolve
	var bodyHTML string
	err = chromedp.Run(taskCtx,
		chromedp.OuterHTML("body", &bodyHTML),
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get body: %w", err)
	}

	// If we see Kasada challenge indicators, wait for the JavaScript to complete
	if strings.Contains(bodyHTML, "KPSDK") || strings.Contains(bodyHTML, "challenge") || len(bodyHTML) < 5000 {
		log.Printf("Detected challenge page, waiting for JavaScript to resolve...")

		// Wait for the Kasada script to load and execute
		// The script should redirect or update the page once verification completes
		for attempt := 0; attempt < 10; attempt++ {
			// Simulate human-like behavior
			err = chromedp.Run(taskCtx,
				chromedp.Evaluate(fmt.Sprintf(`
					(function() {
						var x = %d + Math.random() * 400;
						var y = %d + Math.random() * 300;
						document.dispatchEvent(new MouseEvent('mousemove', {
							clientX: x, clientY: y, bubbles: true
						}));
						window.scrollBy(0, Math.random() * 50);
					})();
				`, 100+attempt*50, 100+attempt*30), nil),
				chromedp.Sleep(2*time.Second),
			)
			if err != nil {
				break
			}

			// Check if we've moved past the challenge
			err = chromedp.Run(taskCtx,
				chromedp.OuterHTML("body", &bodyHTML),
			)
			if err != nil {
				break
			}

			// If body is now larger or doesn't contain KPSDK, we passed!
			if len(bodyHTML) > 5000 && !strings.Contains(bodyHTML, "KPSDK") {
				log.Printf("Challenge resolved after %d attempts!", attempt+1)
				break
			}

			log.Printf("Still on challenge page (attempt %d/10, body length: %d)", attempt+1, len(bodyHTML))
		}
	}

	// Scroll down to trigger lazy loading
	err = chromedp.Run(taskCtx,
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 3)`, nil),
		chromedp.Sleep(randomDelay(500, 1000)),
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 2)`, nil),
		chromedp.Sleep(randomDelay(500, 1000)),
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight)`, nil),
		chromedp.Sleep(randomDelay(1000, 2000)),
		chromedp.Evaluate(`window.scrollTo(0, 0)`, nil),
		chromedp.Sleep(randomDelay(500, 1000)),

		// Get the full page HTML
		chromedp.OuterHTML("html", &html),
		chromedp.Location(&pageURL),
	)
	if err != nil {
		return nil, false, fmt.Errorf("failed to get page content: %w", err)
	}

	log.Printf("Page loaded, URL: %s, HTML length: %d", pageURL, len(html))

	// Final check if we're still blocked
	if strings.Contains(html, "KPSDK") && len(html) < 10000 {
		// Log first 500 chars to see what we're getting
		preview := html
		if len(preview) > 500 {
			preview = preview[:500]
		}
		log.Printf("Challenge page content: %s", preview)
		return nil, false, fmt.Errorf("still blocked by bot protection (Kasada)")
	}

	// Also check for access denied messages
	if strings.Contains(html, "Access Denied") || strings.Contains(html, "403 Forbidden") {
		return nil, false, fmt.Errorf("access denied by server")
	}

	// Parse the HTML to extract listings
	listings, hasMore := s.parseListingsPage(html, propertyType)

	return listings, hasMore, nil
}

// parseListingsPage extracts property data from the HTML page
func (s *BrowserScraper) parseListingsPage(html, propertyType string) ([]models.Property, bool) {
	var listings []models.Property

	// Try multiple JSON extraction patterns
	jsonPatterns := []string{
		// Primary: ArgonautExchange (main data store)
		`window\.ArgonautExchange\s*=\s*(\{.+?\});?\s*</script>`,
		// Alternative: __NEXT_DATA__ (Next.js pages)
		`<script[^>]*id="__NEXT_DATA__"[^>]*>(\{.+?\})</script>`,
		// Alternative: Initial state
		`window\.__INITIAL_STATE__\s*=\s*(\{.+?\});?\s*</script>`,
	}

	for _, pattern := range jsonPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(html)
		if len(matches) >= 2 {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
				listings = s.extractListingsFromJSON(data, propertyType)
				if len(listings) > 0 {
					log.Printf("Extracted %d listings from JSON (pattern: %s)", len(listings), pattern[:30])
					break
				}
			}
		}
	}

	// If JSON extraction didn't work, fall back to HTML parsing
	if len(listings) == 0 {
		listings = s.parseListingCards(html, propertyType)
		if len(listings) > 0 {
			log.Printf("Extracted %d listings from HTML cards", len(listings))
		}
	}

	// Check if there are more pages using multiple indicators
	hasMore := strings.Contains(html, `rel="next"`) ||
		strings.Contains(html, `aria-label="Go to next page"`) ||
		strings.Contains(html, `data-testid="paginator-next-page"`) ||
		strings.Contains(html, `aria-label="Go to Next Page"`) ||
		regexp.MustCompile(`data-testid="[^"]*next[^"]*"`).MatchString(html) ||
		regexp.MustCompile(`class="[^"]*pagination[^"]*"[^>]*>.*?Next`).MatchString(html)

	return listings, hasMore
}

// parseListingCards extracts listings from HTML listing cards
func (s *BrowserScraper) parseListingCards(html, propertyType string) []models.Property {
	var listings []models.Property

	// Find all property links with multiple patterns
	linkPatterns := []*regexp.Regexp{
		regexp.MustCompile(`href="(/property-[^"]+)"`),
		regexp.MustCompile(`href='(/property-[^']+)'`),
		regexp.MustCompile(`data-url="(/property-[^"]+)"`),
	}

	seenIDs := make(map[string]bool)
	listingIDPattern := regexp.MustCompile(`-(\d{6,})$`)

	for _, linkPattern := range linkPatterns {
		links := linkPattern.FindAllStringSubmatch(html, -1)
		for _, link := range links {
			if len(link) < 2 {
				continue
			}

			path := link[1]

			// Extract listing ID (6+ digits at end of URL)
			idMatches := listingIDPattern.FindStringSubmatch(path)
			if len(idMatches) < 2 {
				continue
			}

			listingID := idMatches[1]
			if seenIDs[listingID] {
				continue
			}
			seenIDs[listingID] = true

			// Parse the URL to extract address info
			listing := s.parseListingURL(path, listingID, propertyType)
			if listing != nil {
				listings = append(listings, *listing)
			}
		}
	}

	// Also try to extract listing data from data attributes
	dataPattern := regexp.MustCompile(`data-listing-id="(\d+)"`)
	dataMatches := dataPattern.FindAllStringSubmatch(html, -1)
	for _, match := range dataMatches {
		if len(match) < 2 {
			continue
		}
		listingID := match[1]
		if seenIDs[listingID] {
			continue
		}
		seenIDs[listingID] = true

		// Try to find the URL for this listing
		urlPattern := regexp.MustCompile(fmt.Sprintf(`href="(/property-[^"]*%s)"`, listingID))
		urlMatch := urlPattern.FindStringSubmatch(html)
		path := ""
		if len(urlMatch) >= 2 {
			path = urlMatch[1]
		} else {
			// Construct a generic URL
			path = fmt.Sprintf("/property-%s-nsw-%s", propertyType, listingID)
		}

		listing := s.parseListingURL(path, listingID, propertyType)
		if listing != nil {
			listings = append(listings, *listing)
		}
	}

	return listings
}

// parseListingURL extracts info from a property URL
func (s *BrowserScraper) parseListingURL(path, listingID, propertyType string) *models.Property {
	now := time.Now()
	listing := &models.Property{
		ExternalID:   listingID,
		Source:       "rea",
		URL:          "https://www.realestate.com.au" + path,
		State:        "NSW",
		ScrapedAt:    now,
		UpdatedAt:    now,
		PropertyType: sql.NullString{String: propertyType, Valid: true},
	}

	// Try to extract postcode (4 digits before the ID)
	postcodePattern := regexp.MustCompile(`-(\d{4})-\d+$`)
	if matches := postcodePattern.FindStringSubmatch(path); len(matches) > 1 {
		listing.Postcode = sql.NullString{String: matches[1], Valid: true}
	}

	// Extract suburb (word before -nsw-)
	suburbPattern := regexp.MustCompile(`-([a-z][a-z\+]+)-nsw-\d{4}`)
	if matches := suburbPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		suburb := strings.ReplaceAll(matches[1], "+", " ")
		listing.Suburb = sql.NullString{String: toTitleCase(suburb), Valid: true}
	}

	// Extract street address (between property type and suburb)
	// Pattern: /property-rural-123+example+street-suburb-nsw-2000-12345678
	addressPattern := regexp.MustCompile(`/property-[^/]+-([^/]+)-[a-z]+-nsw-\d{4}-\d+$`)
	if matches := addressPattern.FindStringSubmatch(strings.ToLower(path)); len(matches) > 1 {
		addr := strings.ReplaceAll(matches[1], "+", " ")
		addr = strings.ReplaceAll(addr, "-", " ")
		listing.Address = sql.NullString{String: toTitleCase(addr), Valid: true}
	}

	return listing
}

// extractListingsFromJSON extracts listings from the parsed JSON data
func (s *BrowserScraper) extractListingsFromJSON(data map[string]interface{}, propertyType string) []models.Property {
	var listings []models.Property

	// Try multiple JSON structures that REA uses

	// Structure 1: rpiResults.tieredResults[].results[] (most common)
	if rpi, ok := data["rpiResults"].(map[string]interface{}); ok {
		if tiered, ok := rpi["tieredResults"].([]interface{}); ok {
			for _, tier := range tiered {
				if tierMap, ok := tier.(map[string]interface{}); ok {
					if results, ok := tierMap["results"].([]interface{}); ok {
						for _, result := range results {
							if listing := s.parseJSONListing(result, propertyType); listing != nil {
								listings = append(listings, *listing)
							}
						}
					}
				}
			}
		}
	}

	// Structure 2: searchResults.results[] (alternative format)
	if len(listings) == 0 {
		if sr, ok := data["searchResults"].(map[string]interface{}); ok {
			if results, ok := sr["results"].([]interface{}); ok {
				for _, result := range results {
					if listing := s.parseJSONListing(result, propertyType); listing != nil {
						listings = append(listings, *listing)
					}
				}
			}
		}
	}

	// Structure 3: props.pageProps.listingData (Next.js structure)
	if len(listings) == 0 {
		if props, ok := data["props"].(map[string]interface{}); ok {
			if pageProps, ok := props["pageProps"].(map[string]interface{}); ok {
				// Try listingData.results
				if listingData, ok := pageProps["listingData"].(map[string]interface{}); ok {
					if results, ok := listingData["results"].([]interface{}); ok {
						for _, result := range results {
							if listing := s.parseJSONListing(result, propertyType); listing != nil {
								listings = append(listings, *listing)
							}
						}
					}
				}
				// Try searchResults directly
				if searchResults, ok := pageProps["searchResults"].(map[string]interface{}); ok {
					if tiered, ok := searchResults["tieredResults"].([]interface{}); ok {
						for _, tier := range tiered {
							if tierMap, ok := tier.(map[string]interface{}); ok {
								if results, ok := tierMap["results"].([]interface{}); ok {
									for _, result := range results {
										if listing := s.parseJSONListing(result, propertyType); listing != nil {
											listings = append(listings, *listing)
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Structure 4: Recursive search for listings array
	if len(listings) == 0 {
		listings = s.findListingsRecursive(data, propertyType, 0)
	}

	return listings
}

// findListingsRecursive searches for listings arrays in nested JSON
func (s *BrowserScraper) findListingsRecursive(data interface{}, propertyType string, depth int) []models.Property {
	if depth > 10 { // Prevent infinite recursion
		return nil
	}

	var listings []models.Property

	switch v := data.(type) {
	case map[string]interface{}:
		// Check if this looks like a listing
		if id, hasID := v["id"]; hasID {
			if _, hasURL := v["prettyUrl"]; hasURL {
				if listing := s.parseJSONListing(v, propertyType); listing != nil {
					return []models.Property{*listing}
				}
			}
			if _, hasURL := v["_links"]; hasURL {
				if listing := s.parseJSONListing(v, propertyType); listing != nil {
					return []models.Property{*listing}
				}
			}
			// Check if id is a string that looks like a listing ID
			if idStr, ok := id.(string); ok && len(idStr) >= 6 {
				if listing := s.parseJSONListing(v, propertyType); listing != nil {
					return []models.Property{*listing}
				}
			}
		}

		// Recursively search nested objects
		for key, val := range v {
			// Skip keys that are unlikely to contain listings
			if key == "tracking" || key == "analytics" || key == "meta" {
				continue
			}
			found := s.findListingsRecursive(val, propertyType, depth+1)
			listings = append(listings, found...)
		}

	case []interface{}:
		for _, item := range v {
			found := s.findListingsRecursive(item, propertyType, depth+1)
			listings = append(listings, found...)
		}
	}

	return listings
}

func (s *BrowserScraper) parseJSONListing(data interface{}, propertyType string) *models.Property {
	m, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	now := time.Now()
	listing := &models.Property{
		Source:       "rea",
		State:        "NSW",
		ScrapedAt:    now,
		UpdatedAt:    now,
		PropertyType: sql.NullString{String: propertyType, Valid: true},
	}

	// Extract ID (try multiple field names)
	if id, ok := m["id"].(string); ok && id != "" {
		listing.ExternalID = id
	} else if id, ok := m["listingId"].(string); ok && id != "" {
		listing.ExternalID = id
	} else if id, ok := m["id"].(float64); ok {
		listing.ExternalID = strconv.FormatInt(int64(id), 10)
	}

	if listing.ExternalID == "" {
		return nil
	}

	// Extract URL (try multiple patterns)
	if prettyUrl, ok := m["prettyUrl"].(string); ok {
		if strings.HasPrefix(prettyUrl, "/") {
			listing.URL = "https://www.realestate.com.au" + prettyUrl
		} else {
			listing.URL = prettyUrl
		}
	} else if link, ok := m["_links"].(map[string]interface{}); ok {
		if canonical, ok := link["canonical"].(map[string]interface{}); ok {
			if href, ok := canonical["href"].(string); ok {
				listing.URL = href
			}
		}
	} else if url, ok := m["url"].(string); ok {
		listing.URL = url
	}

	// If no URL, construct from ID
	if listing.URL == "" && listing.ExternalID != "" {
		listing.URL = "https://www.realestate.com.au/property-" + propertyType + "-nsw-" + listing.ExternalID
	}

	// Extract address (try multiple structures)
	if address, ok := m["address"].(map[string]interface{}); ok {
		// Try display.shortAddress first
		if display, ok := address["display"].(map[string]interface{}); ok {
			if shortAddr, ok := display["shortAddress"].(string); ok {
				listing.Address = sql.NullString{String: shortAddr, Valid: true}
			}
			if fullAddr, ok := display["fullAddress"].(string); ok && !listing.Address.Valid {
				listing.Address = sql.NullString{String: fullAddr, Valid: true}
			}
		}
		// Try streetAddress directly
		if streetAddr, ok := address["streetAddress"].(string); ok && !listing.Address.Valid {
			listing.Address = sql.NullString{String: streetAddr, Valid: true}
		}
		// Extract suburb
		if suburb, ok := address["suburb"].(string); ok {
			listing.Suburb = sql.NullString{String: suburb, Valid: true}
		}
		// Extract postcode
		if postcode, ok := address["postcode"].(string); ok {
			listing.Postcode = sql.NullString{String: postcode, Valid: true}
		}
		// Extract state
		if state, ok := address["state"].(string); ok {
			listing.State = state
		}

		// Extract coordinates (try multiple locations)
		if location, ok := address["location"].(map[string]interface{}); ok {
			if lat, ok := location["latitude"].(float64); ok {
				listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
			}
			if lng, ok := location["longitude"].(float64); ok {
				listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
			}
		}
	}

	// Try top-level coordinates
	if !listing.Latitude.Valid {
		if lat, ok := m["latitude"].(float64); ok {
			listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
		}
		if lng, ok := m["longitude"].(float64); ok {
			listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
		}
	}

	// Extract price (try multiple patterns)
	if price, ok := m["price"].(map[string]interface{}); ok {
		if display, ok := price["display"].(string); ok {
			listing.PriceText = sql.NullString{String: display, Valid: true}
			// Try to extract numeric value
			if min, max := extractPriceRange(display); min > 0 {
				listing.PriceMin = sql.NullInt64{Int64: min, Valid: true}
				listing.PriceMax = sql.NullInt64{Int64: max, Valid: true}
			}
		}
	} else if priceText, ok := m["priceText"].(string); ok {
		listing.PriceText = sql.NullString{String: priceText, Valid: true}
	}

	// Extract features (try multiple patterns)
	if features, ok := m["generalFeatures"].(map[string]interface{}); ok {
		if beds, ok := features["bedrooms"].(map[string]interface{}); ok {
			if val, ok := beds["value"].(float64); ok {
				listing.Bedrooms = sql.NullInt64{Int64: int64(val), Valid: true}
			}
		}
		if baths, ok := features["bathrooms"].(map[string]interface{}); ok {
			if val, ok := baths["value"].(float64); ok {
				listing.Bathrooms = sql.NullInt64{Int64: int64(val), Valid: true}
			}
		}
	}
	// Try top-level features
	if !listing.Bedrooms.Valid {
		if beds, ok := m["bedrooms"].(float64); ok {
			listing.Bedrooms = sql.NullInt64{Int64: int64(beds), Valid: true}
		}
		if baths, ok := m["bathrooms"].(float64); ok {
			listing.Bathrooms = sql.NullInt64{Int64: int64(baths), Valid: true}
		}
	}

	// Extract land size (try multiple patterns)
	if propertySizes, ok := m["propertySizes"].(map[string]interface{}); ok {
		if land, ok := propertySizes["land"].(map[string]interface{}); ok {
			if size, ok := land["displayValue"].(string); ok {
				if sqm := parseLandSize(size); sqm > 0 {
					listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
				}
			}
			// Try sizeUnit
			if sizeVal, ok := land["value"].(float64); ok {
				if unit, ok := land["sizeUnit"].(map[string]interface{}); ok {
					if unitName, ok := unit["name"].(string); ok {
						if sqm := convertToSqm(sizeVal, unitName); sqm > 0 {
							listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
						}
					}
				}
			}
		}
	}
	// Try top-level landSize
	if !listing.LandSizeSqm.Valid {
		if landSize, ok := m["landSize"].(string); ok {
			if sqm := parseLandSize(landSize); sqm > 0 {
				listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
			}
		}
	}

	// Extract property type from listing if different
	if pt, ok := m["propertyType"].(string); ok && pt != "" {
		listing.PropertyType = sql.NullString{String: pt, Valid: true}
	}

	// Extract description
	if desc, ok := m["description"].(string); ok {
		listing.Description = sql.NullString{String: desc, Valid: true}
	}

	// Extract images (try multiple patterns)
	var images []string
	if media, ok := m["media"].([]interface{}); ok {
		for _, item := range media {
			if img, ok := item.(map[string]interface{}); ok {
				imgType, _ := img["type"].(string)
				if imgType == "photo" || imgType == "image" || imgType == "" {
					if url, ok := img["url"].(string); ok && url != "" {
						images = append(images, url)
					} else if url, ok := img["imageUrl"].(string); ok && url != "" {
						images = append(images, url)
					}
				}
			}
		}
	}
	// Try images array directly
	if len(images) == 0 {
		if imgArr, ok := m["images"].([]interface{}); ok {
			for _, item := range imgArr {
				if url, ok := item.(string); ok && url != "" {
					images = append(images, url)
				} else if img, ok := item.(map[string]interface{}); ok {
					if url, ok := img["url"].(string); ok && url != "" {
						images = append(images, url)
					}
				}
			}
		}
	}
	// Try mainImage
	if len(images) == 0 {
		if mainImg, ok := m["mainImage"].(map[string]interface{}); ok {
			if url, ok := mainImg["url"].(string); ok && url != "" {
				images = append(images, url)
			}
		}
	}

	if len(images) > 0 {
		imgJSON, _ := json.Marshal(images)
		listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
	}

	return listing
}

// extractPriceRange extracts min and max price from a price string
func extractPriceRange(priceText string) (min, max int64) {
	// Clean the string
	priceText = strings.ToLower(priceText)
	priceText = strings.ReplaceAll(priceText, ",", "")
	priceText = strings.ReplaceAll(priceText, "$", "")

	// Try to find numbers
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*([km])?`)
	matches := re.FindAllStringSubmatch(priceText, -1)

	var values []int64
	for _, m := range matches {
		if len(m) >= 2 {
			val, err := strconv.ParseFloat(m[1], 64)
			if err != nil {
				continue
			}
			// Handle multipliers
			if len(m) >= 3 {
				switch m[2] {
				case "k":
					val *= 1000
				case "m":
					val *= 1000000
				}
			}
			// If value is less than 1000, assume it's in thousands
			if val < 1000 {
				val *= 1000
			}
			values = append(values, int64(val))
		}
	}

	if len(values) >= 2 {
		return values[0], values[1]
	} else if len(values) == 1 {
		return values[0], values[0]
	}
	return 0, 0
}

// convertToSqm converts land size to square meters based on unit
func convertToSqm(value float64, unit string) float64 {
	unit = strings.ToLower(unit)
	switch {
	case strings.Contains(unit, "hectare") || unit == "ha":
		return value * 10000
	case strings.Contains(unit, "acre") || unit == "ac":
		return value * 4046.86
	case strings.Contains(unit, "m²") || strings.Contains(unit, "sqm") || unit == "m2":
		return value
	default:
		return value
	}
}

// FetchListingDetails fetches full details for a single listing using browser
func (s *BrowserScraper) FetchListingDetails(ctx context.Context, listingURL string) (*models.Property, error) {
	taskCtx, cancel := chromedp.NewContext(s.allocCtx)
	defer cancel()

	taskCtx, cancel = context.WithTimeout(taskCtx, 45*time.Second)
	defer cancel()

	var html string

	// Navigate with stealth mode
	err := chromedp.Run(taskCtx,
		// Set headers
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers{
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8",
			"Accept-Language": "en-AU,en;q=0.9",
			"Sec-Ch-Ua":       `"Chromium";v="122", "Not(A:Brand";v="24", "Google Chrome";v="122"`,
		}),

		// Inject stealth script
		chromedp.Evaluate(stealthScript(), nil),

		// Navigate
		chromedp.Navigate(listingURL),

		// Wait for page to load
		chromedp.WaitReady("body"),
		chromedp.Sleep(3*time.Second),

		// Scroll to trigger lazy loading
		chromedp.Evaluate(`window.scrollTo(0, document.body.scrollHeight / 2)`, nil),
		chromedp.Sleep(randomDelay(500, 1000)),

		// Get HTML
		chromedp.OuterHTML("html", &html),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load listing page: %w", err)
	}

	// Check for bot protection
	if strings.Contains(html, "KPSDK") && len(html) < 10000 {
		return nil, fmt.Errorf("blocked by bot protection")
	}

	return s.parseListingDetails(html, listingURL)
}

func (s *BrowserScraper) parseListingDetails(html, listingURL string) (*models.Property, error) {
	// Extract the listing ID from URL
	idPattern := regexp.MustCompile(`-(\d+)$`)
	matches := idPattern.FindStringSubmatch(listingURL)
	if len(matches) < 2 {
		return nil, fmt.Errorf("could not extract listing ID from URL")
	}

	now := time.Now()
	listing := &models.Property{
		ExternalID: matches[1],
		Source:     "rea",
		URL:        listingURL,
		State:      "NSW",
		ScrapedAt:  now,
		UpdatedAt:  now,
	}

	// Try to find embedded JSON with full listing data (JSON-LD)
	jsonLDPattern := regexp.MustCompile(`<script[^>]*type="application/ld\+json"[^>]*>(\{[^<]+\})</script>`)
	jsonMatches := jsonLDPattern.FindAllStringSubmatch(html, -1)

	for _, match := range jsonMatches {
		if len(match) < 2 {
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
			continue
		}

		schemaType, _ := data["@type"].(string)

		// Handle RealEstateListing, Residence, or Product types
		if strings.Contains(schemaType, "RealEstateListing") ||
			strings.Contains(schemaType, "Residence") ||
			strings.Contains(schemaType, "Product") {

			if name, ok := data["name"].(string); ok && name != "" {
				listing.Address = sql.NullString{String: name, Valid: true}
			}
			if desc, ok := data["description"].(string); ok && desc != "" {
				listing.Description = sql.NullString{String: desc, Valid: true}
			}
			if addr, ok := data["address"].(map[string]interface{}); ok {
				if locality, ok := addr["addressLocality"].(string); ok {
					listing.Suburb = sql.NullString{String: locality, Valid: true}
				}
				if postcode, ok := addr["postalCode"].(string); ok {
					listing.Postcode = sql.NullString{String: postcode, Valid: true}
				}
				if streetAddr, ok := addr["streetAddress"].(string); ok && !listing.Address.Valid {
					listing.Address = sql.NullString{String: streetAddr, Valid: true}
				}
				if state, ok := addr["addressRegion"].(string); ok {
					listing.State = state
				}
			}
			if geo, ok := data["geo"].(map[string]interface{}); ok {
				if lat, ok := geo["latitude"].(float64); ok {
					listing.Latitude = sql.NullFloat64{Float64: lat, Valid: true}
				} else if lat, ok := geo["latitude"].(string); ok {
					if latF, err := strconv.ParseFloat(lat, 64); err == nil {
						listing.Latitude = sql.NullFloat64{Float64: latF, Valid: true}
					}
				}
				if lng, ok := geo["longitude"].(float64); ok {
					listing.Longitude = sql.NullFloat64{Float64: lng, Valid: true}
				} else if lng, ok := geo["longitude"].(string); ok {
					if lngF, err := strconv.ParseFloat(lng, 64); err == nil {
						listing.Longitude = sql.NullFloat64{Float64: lngF, Valid: true}
					}
				}
			}
			// Extract images from JSON-LD
			if images, ok := data["image"].([]interface{}); ok {
				var imgUrls []string
				for _, img := range images {
					if url, ok := img.(string); ok && url != "" {
						imgUrls = append(imgUrls, url)
					}
				}
				if len(imgUrls) > 0 {
					imgJSON, _ := json.Marshal(imgUrls)
					listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
				}
			} else if imgUrl, ok := data["image"].(string); ok && imgUrl != "" {
				imgJSON, _ := json.Marshal([]string{imgUrl})
				listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
			}
		}
	}

	// Try to extract from ArgonautExchange JSON
	argonautPattern := regexp.MustCompile(`window\.ArgonautExchange\s*=\s*(\{.+?\});?\s*</script>`)
	if argMatches := argonautPattern.FindStringSubmatch(html); len(argMatches) >= 2 {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(argMatches[1]), &data); err == nil {
			// Navigate to listing data
			if listingData := s.findListingsRecursive(data, "", 0); len(listingData) > 0 {
				// Merge data from JSON into listing
				jsonListing := listingData[0]
				if jsonListing.Address.Valid && !listing.Address.Valid {
					listing.Address = jsonListing.Address
				}
				if jsonListing.Suburb.Valid && !listing.Suburb.Valid {
					listing.Suburb = jsonListing.Suburb
				}
				if jsonListing.Postcode.Valid && !listing.Postcode.Valid {
					listing.Postcode = jsonListing.Postcode
				}
				if jsonListing.Latitude.Valid && !listing.Latitude.Valid {
					listing.Latitude = jsonListing.Latitude
				}
				if jsonListing.Longitude.Valid && !listing.Longitude.Valid {
					listing.Longitude = jsonListing.Longitude
				}
				if jsonListing.PriceText.Valid && !listing.PriceText.Valid {
					listing.PriceText = jsonListing.PriceText
				}
				if jsonListing.LandSizeSqm.Valid && !listing.LandSizeSqm.Valid {
					listing.LandSizeSqm = jsonListing.LandSizeSqm
				}
				if jsonListing.Bedrooms.Valid && !listing.Bedrooms.Valid {
					listing.Bedrooms = jsonListing.Bedrooms
				}
				if jsonListing.Bathrooms.Valid && !listing.Bathrooms.Valid {
					listing.Bathrooms = jsonListing.Bathrooms
				}
				if jsonListing.Images.Valid && !listing.Images.Valid {
					listing.Images = jsonListing.Images
				}
				if jsonListing.Description.Valid && !listing.Description.Valid {
					listing.Description = jsonListing.Description
				}
			}
		}
	}

	// Extract price from page using multiple patterns
	pricePatterns := []*regexp.Regexp{
		regexp.MustCompile(`class="[^"]*property-price[^"]*"[^>]*>([^<]+)<`),
		regexp.MustCompile(`data-testid="[^"]*price[^"]*"[^>]*>([^<]+)<`),
		regexp.MustCompile(`<span[^>]*class="[^"]*Price[^"]*"[^>]*>([^<]+)</span>`),
		regexp.MustCompile(`itemprop="price"[^>]*content="([^"]+)"`),
	}

	if !listing.PriceText.Valid {
		for _, pattern := range pricePatterns {
			if priceMatches := pattern.FindStringSubmatch(html); len(priceMatches) > 1 {
				priceText := strings.TrimSpace(priceMatches[1])
				if priceText != "" && priceText != "Contact Agent" {
					listing.PriceText = sql.NullString{String: priceText, Valid: true}
					break
				}
			}
		}
	}

	// Extract land size from page
	if !listing.LandSizeSqm.Valid {
		landPatterns := []*regexp.Regexp{
			regexp.MustCompile(`([\d,.]+)\s*(hectares?|ha|acres?|ac)\b`),
			regexp.MustCompile(`land[:\s]+size[:\s]*([\d,.]+)\s*(m²|sqm|ha|hectares?|acres?)`),
			regexp.MustCompile(`([\d,.]+)\s*(m²|sqm|square\s*m)`),
		}

		for _, pattern := range landPatterns {
			if landMatches := pattern.FindStringSubmatch(strings.ToLower(html)); len(landMatches) >= 3 {
				sizeStr := strings.ReplaceAll(landMatches[1], ",", "")
				if size, err := strconv.ParseFloat(sizeStr, 64); err == nil {
					sqm := convertToSqm(size, landMatches[2])
					if sqm > 0 {
						listing.LandSizeSqm = sql.NullFloat64{Float64: sqm, Valid: true}
						break
					}
				}
			}
		}
	}

	// Extract images if not already found
	if !listing.Images.Valid {
		imgPatterns := []*regexp.Regexp{
			regexp.MustCompile(`"fullUrl"\s*:\s*"(https://[^"]+(?:jpg|jpeg|png|webp)[^"]*)"`),
			regexp.MustCompile(`<img[^>]+src="(https://[^"]+i\.reast[^"]+)"`),
			regexp.MustCompile(`data-src="(https://[^"]+(?:jpg|jpeg|png|webp)[^"]*)"`),
		}

		var images []string
		seenUrls := make(map[string]bool)

		for _, pattern := range imgPatterns {
			imgMatches := pattern.FindAllStringSubmatch(html, 20)
			for _, match := range imgMatches {
				if len(match) >= 2 {
					url := match[1]
					// Clean URL
					url = strings.ReplaceAll(url, "\\u002F", "/")
					if !seenUrls[url] && strings.HasPrefix(url, "http") {
						seenUrls[url] = true
						images = append(images, url)
					}
				}
			}
			if len(images) >= 10 {
				break
			}
		}

		if len(images) > 0 {
			imgJSON, _ := json.Marshal(images)
			listing.Images = sql.NullString{String: string(imgJSON), Valid: true}
		}
	}

	// Extract bedrooms/bathrooms from HTML if not in JSON
	if !listing.Bedrooms.Valid {
		bedPattern := regexp.MustCompile(`(\d+)\s*(?:bed|bedroom|br)`)
		if bedMatches := bedPattern.FindStringSubmatch(strings.ToLower(html)); len(bedMatches) >= 2 {
			if beds, err := strconv.ParseInt(bedMatches[1], 10, 64); err == nil {
				listing.Bedrooms = sql.NullInt64{Int64: beds, Valid: true}
			}
		}
	}

	if !listing.Bathrooms.Valid {
		bathPattern := regexp.MustCompile(`(\d+)\s*(?:bath|bathroom|ba)`)
		if bathMatches := bathPattern.FindStringSubmatch(strings.ToLower(html)); len(bathMatches) >= 2 {
			if baths, err := strconv.ParseInt(bathMatches[1], 10, 64); err == nil {
				listing.Bathrooms = sql.NullInt64{Int64: baths, Valid: true}
			}
		}
	}

	return listing, nil
}

// toTitleCase converts a string to title case
func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// parseLandSizeBrowser converts land size strings to square meters
func parseLandSizeBrowser(sizeStr string) float64 {
	sizeStr = strings.ToLower(strings.TrimSpace(sizeStr))
	sizeStr = strings.ReplaceAll(sizeStr, ",", "")

	// Extract numeric value
	numPattern := regexp.MustCompile(`([\d.]+)`)
	matches := numPattern.FindStringSubmatch(sizeStr)
	if len(matches) < 2 {
		return 0
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	// Convert to square meters based on unit
	switch {
	case strings.Contains(sizeStr, "ha") || strings.Contains(sizeStr, "hectare"):
		return value * 10000
	case strings.Contains(sizeStr, "acre"):
		return value * 4046.86
	case strings.Contains(sizeStr, "m²") || strings.Contains(sizeStr, "sqm") || strings.Contains(sizeStr, "m2"):
		return value
	default:
		return value
	}
}

// Ensure we don't have unused imports
var _ = cdp.Node{}
