#!/usr/bin/env python3
"""
REA Scraper using undetected-chromedriver to bypass Kasada bot protection.

This script is called by the Go scraper and outputs JSON to stdout.

Usage:
    python rea_scraper.py --url "https://www.realestate.com.au/buy/..." --output json
    python rea_scraper.py --pages 5 --region nsw

Requirements:
    pip install undetected-chromedriver selenium
"""

import argparse
import json
import sys
import time
import re
import random
from typing import List, Dict, Any, Optional

try:
    from selenium import webdriver
    from selenium.webdriver.common.by import By
    from selenium.webdriver.support.ui import WebDriverWait
    from selenium.webdriver.support import expected_conditions as EC
    from selenium.common.exceptions import TimeoutException, NoSuchElementException
    from selenium.webdriver.chrome.service import Service
    from selenium.webdriver.chrome.options import Options
    
    # Try to import undetected_chromedriver
    try:
        import undetected_chromedriver as uc
        HAS_UC = True
    except Exception as e:
        HAS_UC = False
        print(f"undetected_chromedriver not available ({e}), using regular selenium", file=sys.stderr)
    
    # Try webdriver_manager for automatic chromedriver management
    try:
        from webdriver_manager.chrome import ChromeDriverManager
        from webdriver_manager.core.os_manager import ChromeType
        HAS_WDM = True
    except ImportError:
        HAS_WDM = False
        
except ImportError as e:
    print(json.dumps({
        "error": f"Missing dependencies: {e}. Install with: pip install selenium webdriver-manager",
        "listings": []
    }))
    sys.exit(1)


def create_driver(headless: bool = True):
    """Create a Chrome driver instance, trying undetected-chromedriver first."""
    
    # Common Chrome options for stealth
    chrome_options = [
        '--no-sandbox',
        '--disable-dev-shm-usage',
        '--disable-blink-features=AutomationControlled',
        '--disable-infobars',
        '--window-size=1920,1080',
        '--lang=en-AU',
        '--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36',
    ]
    
    if headless:
        chrome_options.append('--headless=new')
    
    # Try undetected_chromedriver first (best for bypassing Kasada)
    if HAS_UC:
        try:
            options = uc.ChromeOptions()
            for opt in chrome_options:
                options.add_argument(opt)
            
            import os
            home = os.path.expanduser('~')
            driver = uc.Chrome(
                options=options, 
                use_subprocess=True,
                browser_executable_path=os.path.join(home, 'chromium'),
                driver_executable_path=os.path.join(home, 'chromedriver'),
            )
            driver.set_page_load_timeout(60)
            print("Using undetected_chromedriver", file=sys.stderr)
            return driver
        except Exception as e:
            print(f"undetected_chromedriver failed: {e}", file=sys.stderr)
    
    # Fall back to regular Selenium
    print("Falling back to regular Selenium", file=sys.stderr)
    options = Options()
    for opt in chrome_options:
        options.add_argument(opt)
    
    # Stealth settings
    options.add_experimental_option('excludeSwitches', ['enable-automation'])
    options.add_experimental_option('useAutomationExtension', False)
    
    # Use snap chromium - need to call the snap wrapper
    # Don't set binary_location as snap handles this
    
    # Try system chromedriver first (snap wrapper)
    try:
        service = Service('/snap/bin/chromium.chromedriver')
        driver = webdriver.Chrome(service=service, options=options)
        print("Using snap chromedriver", file=sys.stderr)
    except Exception as e:
        print(f"System chromedriver failed: {e}", file=sys.stderr)
        if HAS_WDM:
            try:
                service = Service(ChromeDriverManager(chrome_type=ChromeType.CHROMIUM).install())
                driver = webdriver.Chrome(service=service, options=options)
            except Exception as e2:
                print(f"webdriver-manager also failed: {e2}", file=sys.stderr)
                raise
        else:
            raise
    
    # Execute stealth scripts
    driver.execute_cdp_cmd('Page.addScriptToEvaluateOnNewDocument', {
        'source': '''
            Object.defineProperty(navigator, 'webdriver', {get: () => undefined});
            Object.defineProperty(navigator, 'plugins', {get: () => [1, 2, 3, 4, 5]});
            Object.defineProperty(navigator, 'languages', {get: () => ['en-AU', 'en']});
            window.chrome = {runtime: {}};
        '''
    })
    
    driver.set_page_load_timeout(60)
    return driver


def random_delay(min_sec: float = 1.0, max_sec: float = 3.0):
    """Add a random delay to appear more human."""
    time.sleep(random.uniform(min_sec, max_sec))


def scroll_page(driver):
    """Scroll the page to simulate human behavior and trigger lazy loading."""
    try:
        # Scroll down gradually
        for i in range(3):
            driver.execute_script(f"window.scrollTo(0, document.body.scrollHeight * {(i+1)/4});")
            time.sleep(0.5 + random.random())
        
        # Scroll back to top
        driver.execute_script("window.scrollTo(0, 0);")
        time.sleep(0.5)
    except Exception:
        pass


def extract_json_data(html: str) -> Optional[Dict]:
    """Extract embedded JSON data from the page."""
    patterns = [
        r'window\.ArgonautExchange\s*=\s*(\{.+?\});?\s*</script>',
        r'<script[^>]*id="__NEXT_DATA__"[^>]*>(\{.+?\})</script>',
    ]
    
    for pattern in patterns:
        match = re.search(pattern, html, re.DOTALL)
        if match:
            try:
                return json.loads(match.group(1))
            except json.JSONDecodeError:
                continue
    
    return None


def extract_listings_from_json(data: Dict) -> List[Dict]:
    """Extract listing data from the JSON structure."""
    listings = []
    
    # Try rpiResults.tieredResults[].results[]
    try:
        rpi = data.get('rpiResults', {})
        tiered = rpi.get('tieredResults', [])
        for tier in tiered:
            results = tier.get('results', [])
            for r in results:
                listing = parse_json_listing(r)
                if listing:
                    listings.append(listing)
    except (KeyError, TypeError):
        pass
    
    # Try recursive search if no results
    if not listings:
        listings = find_listings_recursive(data)
    
    return listings


def find_listings_recursive(data: Any, depth: int = 0) -> List[Dict]:
    """Recursively search for listings in nested JSON."""
    if depth > 10:
        return []
    
    listings = []
    
    if isinstance(data, dict):
        # Check if this looks like a listing
        if 'id' in data and ('prettyUrl' in data or '_links' in data):
            listing = parse_json_listing(data)
            if listing:
                return [listing]
        
        # Recurse into nested objects
        for key, val in data.items():
            if key in ('tracking', 'analytics', 'meta'):
                continue
            listings.extend(find_listings_recursive(val, depth + 1))
    
    elif isinstance(data, list):
        for item in data:
            listings.extend(find_listings_recursive(item, depth + 1))
    
    return listings


def parse_json_listing(data: Dict) -> Optional[Dict]:
    """Parse a single listing from JSON data."""
    listing = {}
    
    # Extract ID
    listing_id = data.get('id') or data.get('listingId')
    if not listing_id:
        return None
    listing['external_id'] = str(listing_id)
    listing['source'] = 'rea'
    
    # Extract URL
    if 'prettyUrl' in data:
        url = data['prettyUrl']
        if url.startswith('/'):
            url = 'https://www.realestate.com.au' + url
        listing['url'] = url
    elif '_links' in data:
        try:
            listing['url'] = data['_links']['canonical']['href']
        except (KeyError, TypeError):
            pass
    
    # Extract address
    addr = data.get('address', {})
    if isinstance(addr, dict):
        display = addr.get('display', {})
        if isinstance(display, dict):
            listing['address'] = display.get('shortAddress') or display.get('fullAddress')
        listing['suburb'] = addr.get('suburb')
        listing['postcode'] = addr.get('postcode')
        listing['state'] = addr.get('state', 'NSW')
        
        # Coordinates
        location = addr.get('location', {})
        if isinstance(location, dict):
            if 'latitude' in location:
                listing['latitude'] = float(location['latitude'])
            if 'longitude' in location:
                listing['longitude'] = float(location['longitude'])
    
    # Extract price
    price = data.get('price', {})
    if isinstance(price, dict):
        listing['price_text'] = price.get('display')
    
    # Extract features
    features = data.get('generalFeatures', {})
    if isinstance(features, dict):
        beds = features.get('bedrooms', {})
        if isinstance(beds, dict) and 'value' in beds:
            listing['bedrooms'] = int(beds['value'])
        baths = features.get('bathrooms', {})
        if isinstance(baths, dict) and 'value' in baths:
            listing['bathrooms'] = int(baths['value'])
    
    # Extract land size
    sizes = data.get('propertySizes', {})
    if isinstance(sizes, dict):
        land = sizes.get('land', {})
        if isinstance(land, dict):
            display_val = land.get('displayValue', '')
            listing['land_size_sqm'] = parse_land_size(display_val)
    
    # Extract property type
    listing['property_type'] = data.get('propertyType', 'rural')
    
    # Extract images
    media = data.get('media', [])
    if isinstance(media, list):
        images = []
        for item in media:
            if isinstance(item, dict):
                if item.get('type') in ('photo', 'image', None):
                    url = item.get('url') or item.get('imageUrl')
                    if url:
                        images.append(url)
        if images:
            listing['images'] = images
    
    return listing


def parse_land_size(size_str: str) -> Optional[float]:
    """Convert land size string to square meters."""
    if not size_str:
        return None
    
    size_str = size_str.lower().replace(',', '')
    
    match = re.search(r'([\d.]+)', size_str)
    if not match:
        return None
    
    value = float(match.group(1))
    
    if 'hectare' in size_str or 'ha' in size_str:
        return value * 10000
    elif 'acre' in size_str:
        return value * 4046.86
    elif 'm²' in size_str or 'sqm' in size_str or 'm2' in size_str:
        return value
    
    return value


def extract_listings_from_html(driver) -> List[Dict]:
    """Extract listings from HTML when JSON is not available."""
    listings = []
    seen_ids = set()
    
    try:
        # Find all property links
        links = driver.find_elements(By.CSS_SELECTOR, 'a[href*="/property-"]')
        
        for link in links:
            href = link.get_attribute('href')
            if not href:
                continue
            
            # Extract listing ID from URL
            match = re.search(r'-(\d{6,})$', href)
            if not match:
                continue
            
            listing_id = match.group(1)
            if listing_id in seen_ids:
                continue
            seen_ids.add(listing_id)
            
            listing = {
                'external_id': listing_id,
                'source': 'rea',
                'url': href,
                'state': 'NSW',
            }
            
            # Try to extract more info from URL
            # Format: /property-type-address-suburb-state-postcode-id
            parts = href.split('/')[-1].split('-')
            if len(parts) >= 4:
                # Try to find postcode (4 digits before ID)
                for i, part in enumerate(parts[:-1]):
                    if re.match(r'^\d{4}$', part):
                        listing['postcode'] = part
                        if i > 0:
                            listing['suburb'] = parts[i-1].replace('+', ' ').title()
                        break
            
            listings.append(listing)
    
    except Exception as e:
        print(f"Error extracting from HTML: {e}", file=sys.stderr)
    
    return listings


def scrape_page(driver, url: str) -> Dict[str, Any]:
    """Scrape a single page and return results."""
    result = {
        'url': url,
        'listings': [],
        'has_more': False,
        'error': None
    }
    
    try:
        driver.get(url)
        random_delay(2, 4)
        
        # Check for Kasada challenge
        page_source = driver.page_source
        if 'KPSDK' in page_source and len(page_source) < 5000:
            # Wait longer for challenge to resolve
            print("Detected Kasada challenge, waiting...", file=sys.stderr)
            time.sleep(10)
            
            # Simulate some mouse movement
            driver.execute_script("""
                document.dispatchEvent(new MouseEvent('mousemove', {
                    clientX: 100 + Math.random() * 400,
                    clientY: 100 + Math.random() * 300
                }));
            """)
            
            time.sleep(5)
            page_source = driver.page_source
            
            if 'KPSDK' in page_source and len(page_source) < 5000:
                result['error'] = 'Blocked by Kasada bot protection'
                return result
        
        # Scroll to load lazy content
        scroll_page(driver)
        random_delay(1, 2)
        
        # Get updated page source
        page_source = driver.page_source
        
        # Try to extract from JSON first
        json_data = extract_json_data(page_source)
        if json_data:
            result['listings'] = extract_listings_from_json(json_data)
        
        # Fall back to HTML extraction
        if not result['listings']:
            result['listings'] = extract_listings_from_html(driver)
        
        # Check for next page
        result['has_more'] = bool(
            re.search(r'rel="next"', page_source) or
            re.search(r'aria-label="Go to [Nn]ext [Pp]age"', page_source) or
            re.search(r'data-testid="[^"]*next[^"]*"', page_source)
        )
        
    except TimeoutException:
        result['error'] = 'Page load timeout'
    except Exception as e:
        result['error'] = str(e)
    
    return result


def scrape_rea(region: str = 'nsw', max_pages: int = 5, headless: bool = True) -> Dict[str, Any]:
    """Scrape REA listings for a region."""
    results = {
        'listings': [],
        'pages_scraped': 0,
        'errors': []
    }
    
    driver = None
    try:
        print(f"Starting undetected Chrome (headless={headless})...", file=sys.stderr)
        driver = create_driver(headless=headless)
        
        seen_ids = set()
        
        for page_num in range(1, max_pages + 1):
            url = f"https://www.realestate.com.au/buy/property-land-acreage-rural-size-100000-in-{region}/list-{page_num}?activeSort=list-date"
            
            print(f"Scraping page {page_num}: {url}", file=sys.stderr)
            
            page_result = scrape_page(driver, url)
            
            if page_result['error']:
                results['errors'].append(f"Page {page_num}: {page_result['error']}")
                if 'Kasada' in page_result['error']:
                    break
                continue
            
            # Deduplicate
            for listing in page_result['listings']:
                lid = listing.get('external_id')
                if lid and lid not in seen_ids:
                    seen_ids.add(lid)
                    results['listings'].append(listing)
            
            results['pages_scraped'] = page_num
            print(f"Found {len(page_result['listings'])} listings on page {page_num}", file=sys.stderr)
            
            if not page_result['has_more']:
                print("No more pages", file=sys.stderr)
                break
            
            # Random delay between pages
            random_delay(3, 6)
        
    except Exception as e:
        results['errors'].append(str(e))
    
    finally:
        if driver:
            try:
                driver.quit()
            except Exception:
                pass
    
    return results


def main():
    parser = argparse.ArgumentParser(description='Scrape REA listings using undetected-chromedriver')
    parser.add_argument('--url', help='Single URL to scrape')
    parser.add_argument('--region', default='nsw', help='Region to scrape (default: nsw)')
    parser.add_argument('--pages', type=int, default=5, help='Max pages to scrape (default: 5)')
    parser.add_argument('--headless', type=bool, default=True, help='Run in headless mode (default: true)')
    parser.add_argument('--no-headless', action='store_true', help='Run with visible browser')
    
    args = parser.parse_args()
    
    headless = not args.no_headless
    
    if args.url:
        # Single URL mode
        driver = None
        try:
            driver = create_driver(headless=headless)
            result = scrape_page(driver, args.url)
            print(json.dumps(result, indent=2))
        finally:
            if driver:
                driver.quit()
    else:
        # Multi-page mode
        result = scrape_rea(
            region=args.region,
            max_pages=args.pages,
            headless=headless
        )
        print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
