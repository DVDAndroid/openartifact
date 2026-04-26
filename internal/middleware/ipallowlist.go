// SPDX-FileCopyrightText: The OpenArtifact Authors
//
// SPDX-License-Identifier: AGPL-3.0-or-later

package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// IPAllowlistMiddleware blocks requests not from allowed CIDRs.
// If allowlist is empty, all IPs are allowed. trusted controls which
// reverse proxies' X-Forwarded-For/X-Real-IP are honored when resolving the
// client IP — see ClientIP.
func IPAllowlistMiddleware(allowedCIDRs []string, trusted *TrustedProxies) func(http.Handler) http.Handler {
	// Parse CIDRs at middleware creation time
	var networks []*net.IPNet

	for _, cidr := range allowedCIDRs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			// Skip invalid CIDRs (log would be ideal but we don't have logger here)
			continue
		}

		networks = append(networks, network)
	}

	// If no valid networks, allow all (passthrough)
	if len(networks) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := ClientIP(r, trusted)
			ip := net.ParseIP(clientIP)

			// Check if IP is in any allowed network
			allowed := false

			if ip != nil {
				for _, network := range networks {
					if network.Contains(ip) {
						allowed = true
						break
					}
				}
			}

			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "IP not allowed"})

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
