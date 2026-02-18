// Author: Toluwalase Mebaanne
// Package auth provides authentication utilities for the TailClip system.
// WHY: Both the hub and agent need a shared, consistent way to validate
// API requests. Centralizing auth logic here prevents inconsistencies
// and ensures security best practices are applied everywhere.

package auth

import (
	"crypto/subtle"
	"net/http"
)

// ValidateToken compares an expected token against a provided token
// using constant-time comparison.
//
// WHY constant-time comparison:
// A naive string comparison (==) short-circuits on the first mismatched byte.
// An attacker can measure response times to determine how many leading bytes
// of their guess match the real token. By trying different values and observing
// timing differences, they can reconstruct the token one byte at a time.
// This is called a "timing attack."
//
// crypto/subtle.ConstantTimeCompare always takes the same amount of time
// regardless of where (or if) the strings differ, eliminating the timing
// side channel entirely.
func ValidateToken(expected, provided string) bool {
	// Guard against empty tokens
	// WHY: An empty expected token means auth is misconfigured - reject everything.
	// An empty provided token means the caller didn't authenticate - always reject.
	if expected == "" || provided == "" {
		return false
	}

	// ConstantTimeCompare requires equal-length byte slices to be truly constant-time.
	// WHY: If lengths differ, the comparison returns 0 immediately, but the length
	// check itself leaks information (attacker learns the token length). However,
	// for bearer-style tokens the length is already public knowledge from the
	// config format, so this is an acceptable trade-off.
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

// ExtractTokenFromHeader retrieves the authentication token from the
// HTTP X-Auth-Token header.
//
// WHY use a custom header instead of standard Authorization:
// X-Auth-Token is simple and avoids the complexity of parsing "Bearer <token>"
// schemes. TailClip uses a shared secret model (not OAuth), so a straightforward
// custom header keeps the implementation clean and easy to debug.
//
// WHY header-based extraction:
// HTTP headers are the standard, secure way to pass credentials in REST APIs.
// Headers are not logged by default in most reverse proxies and web servers,
// reducing the risk of accidental token exposure compared to URL parameters.
// This is the preferred method for standard HTTP requests from the agent.
func ExtractTokenFromHeader(r *http.Request) string {
	return r.Header.Get("X-Auth-Token")
}

// ExtractTokenFromQuery retrieves the authentication token from the
// URL query parameter "token".
//
// WHY support query parameter extraction in addition to headers:
// WebSocket connections (used for real-time clipboard sync) cannot set
// custom HTTP headers during the initial handshake in most browser and
// client implementations. The WebSocket upgrade request is initiated by
// the browser/client via a standard HTTP GET, and custom headers are
// often stripped or unsupported.
//
// Passing the token as a query parameter (?token=<value>) is the widely
// accepted workaround for WebSocket authentication. While query params
// can appear in server logs and browser history, TailClip operates over
// a private Tailscale network, which mitigates this risk.
//
// SECURITY NOTE: Use header-based auth (ExtractTokenFromHeader) for all
// standard HTTP endpoints. Reserve query param auth for WebSocket upgrades only.
func ExtractTokenFromQuery(r *http.Request) string {
	return r.URL.Query().Get("token")
}

// Authenticate checks both header and query parameter for a valid token.
// WHY: Provides a single entry point for request authentication, checking
// the preferred header method first, then falling back to query parameter.
// This simplifies handler code by consolidating the auth check.
func Authenticate(r *http.Request, expectedToken string) bool {
	// Try header first - WHY: Headers are the more secure transport,
	// so prioritize them over query params
	if token := ExtractTokenFromHeader(r); token != "" {
		return ValidateToken(expectedToken, token)
	}

	// Fall back to query parameter - WHY: Supports WebSocket upgrade
	// requests where headers may not be available
	if token := ExtractTokenFromQuery(r); token != "" {
		return ValidateToken(expectedToken, token)
	}

	return false
}
