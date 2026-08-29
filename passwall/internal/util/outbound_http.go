package util

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type outboundResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type restrictedRoundTripper struct {
	base          http.RoundTripper
	resolver      outboundResolver
	resolveTarget bool
}

func (t *restrictedRoundTripper) CloseIdleConnections() {
	if transport, ok := t.base.(interface{ CloseIdleConnections() }); ok {
		transport.CloseIdleConnections()
	}
}

func (t *restrictedRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("outbound request is nil")
	}
	if t.resolveTarget {
		if err := validateOutboundURL(req.Context(), req.URL, t.resolver); err != nil {
			return nil, err
		}
	} else if err := validateOutboundURLStructure(req.URL); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(req)
}

func newRestrictedHTTPClient(timeout time.Duration, proxyURL string, resolver outboundResolver) (*http.Client, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	restrictedTransport := &restrictedRoundTripper{base: transport, resolver: resolver}

	if proxyURL != "" {
		proxy, err := url.Parse(proxyURL)
		if err != nil || proxy.Scheme == "" || proxy.Hostname() == "" {
			return nil, errors.New("invalid proxy URL")
		}
		transport.Proxy = http.ProxyURL(proxy)
		restrictedTransport.resolveTarget = true
	} else {
		transport.Proxy = nil
		dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
		transport.DialContext = restrictedDialContext(resolver, dialer.DialContext)
	}

	return &http.Client{
		Timeout:       timeout,
		Transport:     restrictedTransport,
		CheckRedirect: restrictedRedirectPolicy(resolver),
	}, nil
}

func restrictedRedirectPolicy(resolver outboundResolver) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return validateOutboundURL(req.Context(), req.URL, resolver)
	}
}

func restrictedDialContext(resolver outboundResolver, dial func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("invalid outbound address")
		}
		addresses, err := resolveSafeAddresses(ctx, host, resolver)
		if err != nil {
			return nil, err
		}

		var lastErr error
		for _, resolved := range addresses {
			host := resolved.IP.String()
			if resolved.Zone != "" {
				host += "%" + resolved.Zone
			}
			conn, err := dial(ctx, network, net.JoinHostPort(host, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
		}
		return nil, lastErr
	}
}

func validateOutboundURL(ctx context.Context, target *url.URL, resolver outboundResolver) error {
	if err := validateOutboundURLStructure(target); err != nil {
		return err
	}
	_, err := resolveSafeAddresses(ctx, target.Hostname(), resolver)
	return err
}

func validateOutboundURLStructure(target *url.URL) error {
	if target == nil || target.Hostname() == "" {
		return errors.New("outbound URL must include a host")
	}
	if target.User != nil {
		return errors.New("outbound URL userinfo is not allowed")
	}
	switch strings.ToLower(target.Scheme) {
	case "http", "https":
		return nil
	default:
		return errors.New("outbound URL scheme is not allowed")
	}
}

func resolveSafeAddresses(ctx context.Context, host string, resolver outboundResolver) ([]net.IPAddr, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	if literal, err := netip.ParseAddr(host); err == nil {
		literal = literal.Unmap()
		if prohibitedOutboundAddress(literal) {
			return nil, errors.New("outbound host resolves to a prohibited address")
		}
		return []net.IPAddr{{IP: net.IP(literal.AsSlice()), Zone: literal.Zone()}}, nil
	}

	addresses, err := resolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("outbound host could not be resolved")
	}
	for i := range addresses {
		address, ok := netip.AddrFromSlice(addresses[i].IP)
		if !ok {
			return nil, errors.New("outbound host returned an invalid address")
		}
		address = address.Unmap()
		if prohibitedOutboundAddress(address) {
			return nil, errors.New("outbound host resolves to a prohibited address")
		}
		addresses[i].IP = net.IP(address.AsSlice())
	}
	return addresses, nil
}

var specialOutboundPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("::ffff:0:0:0/96"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("fec0::/10"),
}

func prohibitedOutboundAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsLoopback() || address.IsPrivate() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	for _, prefix := range specialOutboundPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}
