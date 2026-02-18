package api

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

func BodyLimitByPath(defaultMax, adminMax int64, adminPrefix string) func(http.Handler) http.Handler {
	if defaultMax <= 0 && adminMax <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	if adminPrefix == "" {
		adminPrefix = "/"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limit := defaultMax
			if adminMax > 0 && strings.HasPrefix(r.URL.Path, adminPrefix) {
				limit = adminMax
			}
			if limit <= 0 {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > limit && r.ContentLength != -1 {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
			next.ServeHTTP(w, r)
		})
	}
}

func RealIPFromTrustedProxies(trusted []netip.Prefix) func(http.Handler) http.Handler {
	if len(trusted) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			remoteIP, ok := parseRemoteIP(r.RemoteAddr)
			if !ok || !isTrustedIP(remoteIP, trusted) {
				next.ServeHTTP(w, r)
				return
			}

			if ip := forwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
				r.RemoteAddr = ip
				next.ServeHTTP(w, r)
				return
			}

			if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
				if addr, err := netip.ParseAddr(ip); err == nil {
					r.RemoteAddr = addr.String()
					next.ServeHTTP(w, r)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func parseRemoteIP(addr string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return netip.Addr{}, false
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip, true
}

func isTrustedIP(ip netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func forwardedFor(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if ip, err := netip.ParseAddr(part); err == nil {
			return ip.String()
		}
	}
	return ""
}
