package utils

import (
	"net/http"
	"strings"
)

// DeviceInfo holds parsed information about a client's device, platform,
// browser, and email client derived from the User-Agent string.
type DeviceInfo struct {
	DeviceType  string `json:"device_type"`
	Platform    string `json:"platform"`
	Browser     string `json:"browser"`
	EmailClient string `json:"email_client"`
	IsBot       bool   `json:"is_bot"`
}

// botPatterns contains substrings commonly found in bot/crawler user agents.
var botPatterns = []string{
	"bot", "crawler", "spider", "slurp", "mediapartners",
	"curl", "wget", "python", "java", "libhttp", "httpclient",
	"apache", "scrapy", "phantomjs", "headless", "selenium",
}

// emailClientPatterns maps recognizable email client user-agent substrings
// to a human-friendly name.
var emailClientPatterns = map[string]string{
	"microsoft-outlook": "Outlook",
	"outlook":           "Outlook",
	"thunderbird":       "Thunderbird",
	"applemail":         "Apple Mail",
	"mailbird":          "Mailbird",
	"yahoo":             "Yahoo Mail",
	"gmail":             "Gmail",
	"googlemail":        "Gmail",
	"eM Client":         "eM Client",
	"postbox":           "Postbox",
	"airmail":           "Airmail",
	"spark":             "Spark",
	"livemail":          "Windows Live Mail",
}

// ParseUserAgent inspects the given User-Agent string and returns a
// DeviceInfo struct with the detected device type, platform, browser,
// email client, and whether the client is a bot.
func ParseUserAgent(ua string) DeviceInfo {
	info := DeviceInfo{
		DeviceType:  "desktop",
		Platform:    "Unknown",
		Browser:     "Unknown",
		EmailClient: "Unknown",
	}

	lower := strings.ToLower(ua)

	// --- Bot detection ---
	for _, pattern := range botPatterns {
		if strings.Contains(lower, pattern) {
			info.IsBot = true
			break
		}
	}

	// --- Device type ---
	switch {
	case strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet"):
		info.DeviceType = "tablet"
	case strings.Contains(lower, "iphone") || strings.Contains(lower, "ipod"),
		strings.Contains(lower, "mobile") && !strings.Contains(lower, "ipad"):
		info.DeviceType = "mobile"
	case strings.Contains(lower, "android") && !strings.Contains(lower, "mobile"):
		info.DeviceType = "tablet"
	}

	// --- Platform ---
	switch {
	case strings.Contains(lower, "windows"):
		info.Platform = "Windows"
	case strings.Contains(lower, "macintosh"), strings.Contains(lower, "mac os"):
		info.Platform = "macOS"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "ipad"), strings.Contains(lower, "ipod"):
		info.Platform = "iOS"
	case strings.Contains(lower, "android"):
		info.Platform = "Android"
	case strings.Contains(lower, "linux"):
		info.Platform = "Linux"
	}

	// --- Browser ---
	switch {
	case strings.Contains(lower, "edg"):
		info.Browser = "Edge"
	case strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg"):
		info.Browser = "Chrome"
	case strings.Contains(lower, "firefox"):
		info.Browser = "Firefox"
	case strings.Contains(lower, "safari") && !strings.Contains(lower, "chrome") && !strings.Contains(lower, "edg"):
		info.Browser = "Safari"
	case strings.Contains(lower, "trident"), strings.Contains(lower, "msie"):
		info.Browser = "IE"
	}

	// --- Email Client ---
	for pattern, name := range emailClientPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			info.EmailClient = name
			break
		}
	}

	return info
}

// getClientIP extracts the client IP address from an HTTP request.
// It checks the X-Forwarded-For and X-Real-IP headers (for use behind a
// reverse proxy) before falling back to the remote address.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (may contain a comma-separated list)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}
	// Check X-Real-IP header
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		return strings.TrimSpace(xrip)
	}
	// Fall back to RemoteAddr, stripping the port if present
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}

// getTLSVersion returns the TLS version string for the given HTTP request.
// Returns an empty string if the request did not use TLS.
func getTLSVersion(r *http.Request) string {
	if r.TLS == nil {
		return ""
	}
	switch r.TLS.Version {
	case 0x0301:
		return "TLS 1.0"
	case 0x0302:
		return "TLS 1.1"
	case 0x0303:
		return "TLS 1.2"
	case 0x0304:
		return "TLS 1.3"
	default:
		return "Unknown"
	}
}
