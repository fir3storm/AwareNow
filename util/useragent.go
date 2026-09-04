/*
gophish - Open-Source Phishing Framework

The MIT License (MIT)

Copyright (c) 2013 Jordan Wright

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/

package util

import (
	"regexp"
	"strings"
)

// BrowserInfo holds parsed browser and OS information from a User-Agent string.
type BrowserInfo struct {
	Browser         string
	BrowserVersion  string
	OS              string
	OSVersion       string
	DeviceType      string
	Platform        string
	IsMobile        bool
	IsBot           bool
	EmailClient     string
}

var (
	// browserRegex matches browser name and version patterns
	browserRegex = regexp.MustCompile(`(?i)(firefox|chrome|safari|edge|edg|opera|opr|msie|trident)[\/\s]*([\d.]+)?`)
	// osRegex matches OS patterns
	osRegex = regexp.MustCompile(`(?i)(windows nt|mac os x|linux|android|iphone|ipad|ios)[\/\s]*([\d._]+)?`)
	// botRegex matches known bot/crawler patterns
	botRegex = regexp.MustCompile(`(?i)(bot|crawler|spider|slurp|googlebot|bingbot|yandex|baiduspider)`)
	// tabletRegex matches tablet device patterns
	tabletRegex = `(?i)(tablet|ipad|kindle|silk|playbook)`
	// mobileRegex matches mobile device patterns
	mobileRegex = `(?i)(mobile|iphone|ipod|android.*mobile|windows phone)`
)

// ParseUserAgent parses a raw User-Agent string and returns structured browser info.
func ParseUserAgent(rawUA string) *BrowserInfo {
	info := &BrowserInfo{
		Browser:    "Unknown",
		OS:         "Unknown",
		DeviceType: "desktop",
	}

	if rawUA == "" {
		return info
	}

	ua := strings.ToLower(rawUA)

	// Detect bot
	if botRegex.MatchString(ua) {
		info.IsBot = true
		info.DeviceType = "bot"
	}

	// Detect OS
	if match := osRegex.FindStringSubmatch(ua); len(match) >= 2 {
		info.OS = normalizeOS(match[1])
		if len(match) >= 3 && match[2] != "" {
			info.OSVersion = strings.ReplaceAll(match[2], "_", ".")
		}
	}

	// Detect browser
	if match := browserRegex.FindStringSubmatch(rawUA); len(match) >= 2 {
		info.Browser = normalizeBrowser(match[1])
		if len(match) >= 3 && match[2] != "" {
			info.BrowserVersion = match[2]
		}
	}

	// Determine device type
	info.DeviceType = determineDeviceType(ua, info)
	info.IsMobile = info.DeviceType == "mobile" || info.DeviceType == "tablet"

	// Set platform
	info.OS = strings.Title(info.OS)

	// Detect email client
	info.EmailClient = detectEmailClient(ua, info)

	return info
}

// GetDeviceType determines the device type (desktop, mobile, tablet, bot)
// from a User-Agent string.
func GetDeviceType(rawUA string) string {
	info := ParseUserAgent(rawUA)
	return info.DeviceType
}

// GetEmailClient detects the email client from a User-Agent string.
func GetEmailClient(rawUA string) string {
	info := ParseUserAgent(rawUA)
	return info.EmailClient
}

// GetOSInfo extracts the operating system name from a User-Agent string.
func GetOSInfo(rawUA string) string {
	info := ParseUserAgent(rawUA)
	return info.OS
}

// GetBrowserName extracts the browser name from a User-Agent string.
func GetBrowserName(rawUA string) string {
	info := ParseUserAgent(rawUA)
	return info.Browser
}

// GetBrowserInfo returns a map containing comprehensive device information
// parsed from a User-Agent string.
func GetBrowserInfo(rawUA string) map[string]string {
	info := ParseUserAgent(rawUA)
	return map[string]string{
		"browser":          info.Browser,
		"browser_version":  info.BrowserVersion,
		"os":               info.OS,
		"os_version":       info.OSVersion,
		"device_type":      info.DeviceType,
		"platform":         info.Platform,
		"email_client":     info.EmailClient,
	}
}

// normalizeOS normalizes OS names from User-Agent strings.
func normalizeOS(os string) string {
	os = strings.ToLower(strings.TrimSpace(os))
	switch {
	case strings.Contains(os, "windows nt"):
		return "Windows"
	case strings.Contains(os, "mac os x"), strings.Contains(os, "macos"):
		return "macOS"
	case strings.Contains(os, "linux"):
		return "Linux"
	case strings.Contains(os, "android"):
		return "Android"
	case strings.Contains(os, "iphone"):
		return "iOS"
	case strings.Contains(os, "ipad"):
		return "iPadOS"
	case strings.Contains(os, "ios"):
		return "iOS"
	default:
		return strings.Title(os)
	}
}

// normalizeBrowser normalizes browser names from User-Agent strings.
func normalizeBrowser(browser string) string {
	browser = strings.ToLower(strings.TrimSpace(browser))
	switch {
	case browser == "firefox":
		return "Firefox"
	case browser == "chrome":
		return "Chrome"
	case browser == "safari":
		return "Safari"
	case browser == "edge", browser == "edg":
		return "Edge"
	case browser == "opera", browser == "opr":
		return "Opera"
	case browser == "msie", browser == "trident":
		return "Internet Explorer"
	default:
		return strings.Title(browser)
	}
}

// determineDeviceType determines the device type from the User-Agent.
func determineDeviceType(ua string, info *BrowserInfo) string {
	if info.IsBot {
		return "bot"
	}
	tabletRe := regexp.MustCompile(tabletRegex)
	if tabletRe.MatchString(ua) {
		return "tablet"
	}
	mobileRe := regexp.MustCompile(mobileRegex)
	if mobileRe.MatchString(ua) {
		return "mobile"
	}
	return "desktop"
}

// detectEmailClient attempts to identify the email client from the User-Agent.
func detectEmailClient(ua string, info *BrowserInfo) string {
	switch {
	case strings.Contains(ua, "microsoft outlook"), strings.Contains(ua, "ms-office"):
		return "Microsoft Outlook"
	case strings.Contains(ua, "msoutlook"):
		return "Microsoft Outlook"
	case strings.Contains(ua, "macintosh"), strings.Contains(ua, "mac os x"):
		// Check for Apple Mail specific patterns
		if strings.Contains(ua, "applewebkit") && !strings.Contains(ua, "chrome") {
			return "Apple Mail"
		}
		return "Apple Mail (or Safari)"
	case strings.Contains(ua, "googleimageproxy"), strings.Contains(ua, "google image"):
		return "Gmail"
	case strings.Contains(ua, "gmail"):
		return "Gmail"
	case strings.Contains(ua, "yahoo"), strings.Contains(ua, "yahoomail"):
		return "Yahoo Mail"
	case strings.Contains(ua, "thunderbird"):
		return "Mozilla Thunderbird"
	case strings.Contains(ua, "ios-"):
		return "iOS Mail"
	case strings.Contains(ua, "android") && strings.Contains(ua, "gmail"):
		return "Gmail (Android)"
	default:
		// Fall back to browser detection for webmail
		if info.Browser != "Unknown" {
			return "Webmail (" + info.Browser + ")"
		}
		return "Unknown"
	}
}
