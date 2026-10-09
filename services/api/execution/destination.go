package execution

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Deliberately conservative special-use policy, grounded in the IANA registries
// checked 2026-10-08. IPv6 is limited to ordinary 2000::/3 global assignments.
var deniedPrefixes = func() []netip.Prefix {
	out := []netip.Prefix{}
	for _, p := range []string{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.52.193.0/24", "192.88.99.0/24", "192.168.0.0/16", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		out = append(out, netip.MustParsePrefix(p))
	}
	return out
}()

func PublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || !ip.IsGlobalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, p := range deniedPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

var hostPattern = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)
var numericHost = regexp.MustCompile(`^[0-9.]+$`)

func ValidateDestination(raw string) (*url.URL, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(raw) > ConfigLimit {
		return nil, failure(400, "destination_blocked")
	}
	port := u.Port()
	if port != "" && port != "80" && port != "443" {
		return nil, failure(400, "destination_blocked")
	}
	h := u.Hostname()
	if ip, e := netip.ParseAddr(h); e == nil {
		if !PublicIP(ip) {
			return nil, failure(400, "destination_blocked")
		}
	} else if !hostPattern.MatchString(h) || numericHost.MatchString(h) || strings.HasPrefix(strings.ToLower(h), "0x") || strings.HasSuffix(h, ".") || len(h) > 253 || strings.EqualFold(h, "localhost") {
		return nil, failure(400, "destination_blocked")
	}
	return u, nil
}

type Resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

func validatedIPs(ctx context.Context, r Resolver, host string) ([]netip.Addr, error) {
	if ip, e := netip.ParseAddr(host); e == nil {
		if !PublicIP(ip) {
			return nil, failure(400, "destination_blocked")
		}
		return []netip.Addr{ip.Unmap()}, nil
	}
	dns, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	ips, e := r.LookupNetIP(dns, "ip", host)
	if e != nil || len(ips) == 0 {
		return nil, failure(502, "upstream_network_error")
	}
	for i, ip := range ips {
		if !PublicIP(ip) {
			return nil, failure(400, "destination_blocked")
		}
		ips[i] = ip.Unmap()
	}
	return ips, nil
}
func safeDialer(r Resolver) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil || (port != "80" && port != "443") {
			return nil, failure(400, "destination_blocked")
		}
		ips, e := validatedIPs(ctx, r, host)
		if e != nil {
			return nil, e
		}
		dial := net.Dialer{Timeout: 5 * time.Second}
		return dial.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}
