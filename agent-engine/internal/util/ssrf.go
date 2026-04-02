package util

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// SSRFError is returned when a URL is blocked by the SSRF guard.
type SSRFError struct {
	URL    string
	Reason string
}

func (e *SSRFError) Error() string {
	return fmt.Sprintf("SSRF guard: %s blocked (%s)", e.URL, e.Reason)
}

// blockedHosts are hostnames that must never be reachable from agent-issued
// HTTP calls.  These cover the common cloud metadata endpoints.
var blockedHosts = []string{
	"169.254.169.254",         // AWS / GCP / Azure IMDS
	"metadata.google.internal",
	"169.254.170.2",           // AWS ECS metadata
	"100.100.100.200",         // Alibaba Cloud metadata
	"192.0.0.192",             // Oracle Cloud metadata
}

// blockedCIDRs are private / link-local address ranges that outbound agent
// HTTP calls must not reach.
var blockedCIDRs []*net.IPNet

func init() {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"::1/128",
		"fc00::/7",   // IPv6 ULA
		"fe80::/10",  // IPv6 link-local
		"169.254.0.0/16", // IPv4 link-local
	} {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			blockedCIDRs = append(blockedCIDRs, ipNet)
		}
	}
}

// CheckSSRF validates that the given rawURL is safe to fetch from agent code.
// Returns nil if the URL is allowed, or an *SSRFError if it should be blocked.
//
// Checks performed:
//   - Scheme must be http or https
//   - Host must not be a known metadata endpoint
//   - Host must not resolve to a private / loopback address
func CheckSSRF(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return &SSRFError{URL: rawURL, Reason: "invalid URL: " + err.Error()}
	}

	// Scheme check.
	if u.Scheme != "http" && u.Scheme != "https" {
		return &SSRFError{URL: rawURL, Reason: "scheme " + u.Scheme + " not permitted"}
	}

	host := u.Hostname()
	if host == "" {
		return &SSRFError{URL: rawURL, Reason: "empty host"}
	}

	// Blocked hostname list.
	for _, blocked := range blockedHosts {
		if strings.EqualFold(host, blocked) {
			return &SSRFError{URL: rawURL, Reason: "host is a blocked metadata endpoint"}
		}
	}

	// Resolve the host and check each returned IP against blocked CIDRs.
	addrs, err := net.LookupHost(host)
	if err != nil {
		// If DNS fails we cannot validate — block conservatively.
		return &SSRFError{URL: rawURL, Reason: "DNS resolution failed: " + err.Error()}
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		for _, cidr := range blockedCIDRs {
			if cidr.Contains(ip) {
				return &SSRFError{
					URL:    rawURL,
					Reason: fmt.Sprintf("resolved IP %s is in blocked range %s", ip, cidr),
				}
			}
		}
	}

	return nil
}

// MustCheckSSRF is like CheckSSRF but panics on block.  Use only in tests.
func MustCheckSSRF(rawURL string) {
	if err := CheckSSRF(rawURL); err != nil {
		panic(err)
	}
}
