package singbox

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/luynrs/justray/internal/daemon/engine/singbox/resolvers"
	"github.com/luynrs/justray/internal/shared/domain"
)

const Tag = "proxy"

func packetEncoding(n domain.Node) string { return cmp.Or(n.PacketEncoding, "xudp") }

func Build(n domain.Node, s domain.Settings, logPath string, tun bool) (*option.Options, error) {
	ep, obs, err := Proxy(n, s)
	if err != nil {
		return nil, err
	}

	resolverIPs := resolvers.Get()
	resolverCIDRs := make([]string, 0, len(resolverIPs))
	for _, p := range resolverIPs {
		resolverCIDRs = append(resolverCIDRs, p.String())
	}

	opts := &option.Options{
		Log: &option.LogOptions{Level: s.LogLevel, Output: logPath},
		Inbounds: []option.Inbound{
			{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{
				ListenOptions: option.ListenOptions{Listen: addr("127.0.0.1"), ListenPort: uint16(s.Port)},
			}},
		},
		Outbounds: []option.Outbound{
			{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}},
		},
		DNS: &option.DNSOptions{RawDNSOptions: option.RawDNSOptions{
			DNSClientOptions: option.DNSClientOptions{Strategy: dnsStrategy[s.IPVersion]},
			Servers: []option.DNSServerOptions{
				{Type: C.DNSTypeTCP, Tag: "remote", Options: &option.RemoteDNSServerOptions{
					RawLocalDNSServerOptions: option.RawLocalDNSServerOptions{
						DialerOptions: option.DialerOptions{Detour: strings.TrimPrefix(final(s), "direct")}, // a detour to a bare direct outbound is rejected by sing-box
					},
					DNSServerAddressOptions: option.DNSServerAddressOptions{Server: s.DNS},
				}},
			}}},
		Route: &option.RouteOptions{
			Final:               final(s),
			AutoDetectInterface: true,
			Rules:               rules(s, resolverCIDRs),
		},
	}
	if ep != nil {
		opts.Endpoints = append(opts.Endpoints, *ep)
	}
	opts.Outbounds = append(opts.Outbounds, obs...)
	if tun {
		opts.Inbounds = append(opts.Inbounds, TunInbound(s, resolverIPs))
	}
	return opts, nil
}

func Proxy(n domain.Node, s domain.Settings) (*option.Endpoint, []option.Outbound, error) {
	n, err := resolved(n, s)
	if err != nil {
		return nil, nil, err
	}
	var scratch option.Options
	if err := add(&scratch, n, Tag); err != nil {
		return nil, nil, err
	}
	var ep *option.Endpoint
	if len(scratch.Endpoints) > 0 {
		ep = &scratch.Endpoints[0]
	}
	return ep, scratch.Outbounds, nil
}

// BlockConfig is a tun that rejects everything
func BlockConfig(s domain.Settings, logPath string) *option.Options {
	return &option.Options{
		Log:      &option.LogOptions{Level: s.LogLevel, Output: logPath},
		Inbounds: []option.Inbound{TunInbound(s, resolvers.Get())},
		Route: &option.RouteOptions{
			AutoDetectInterface: true,
			Rules: []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{Inbound: []string{"tun-in"}},
				RuleAction:     reject,
			}}},
		},
	}
}

var reject = option.RuleAction{
	Action:        C.RuleActionTypeReject,
	RejectOptions: option.RejectActionOptions{Method: C.RuleActionRejectMethodDefault},
}

func match(list []string, action option.RuleAction) []option.Rule {
	cidrs, domains, keywords, names, paths := domain.SplitRules(list)

	var out []option.Rule
	for _, m := range []option.RawDefaultRule{
		{ProcessName: names}, {ProcessPath: paths},
		{IPCIDR: cidrs}, {DomainSuffix: domains}, {DomainKeyword: keywords},
	} {
		if len(m.ProcessName)+len(m.ProcessPath)+len(m.IPCIDR)+len(m.DomainSuffix)+len(m.DomainKeyword) == 0 {
			continue
		}
		out = append(out, option.Rule{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: m,
			RuleAction:     action,
		}})
	}
	return out
}

func rules(s domain.Settings, direct []string) []option.Rule {
	// a TUN connection carries only an address, a mixed-in one only a domain
	out := []option.Rule{
		{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RuleAction: option.RuleAction{Action: C.RuleActionTypeSniff}}},
		{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{RuleAction: option.RuleAction{Action: C.RuleActionTypeResolve}}},
	}

	if s.DNSHijack == "on" {
		out = append(out, option.Rule{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{Port: []uint16{53}},
			RuleAction:     option.RuleAction{Action: C.RuleActionTypeHijackDNS},
		}})
	}
	if s.BlockQUIC == "on" {
		out = append(out, option.Rule{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{
				Network: []string{"udp"},
				Port:    []uint16{443},
			},
			RuleAction: reject,
		}})
	}
	toDirect := option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "direct"}}
	toProxy := option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: Tag}}

	toExcept := toDirect
	if s.Mode == domain.DirectAll {
		toExcept = toProxy
	}

	out = append(out, match(s.Blocked, reject)...)

	if s.BypassLocal == "on" {
		out = append(out, option.Rule{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{IPIsPrivate: true},
			RuleAction:     toDirect,
		}})
	}
	out = append(out, option.Rule{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
		RawDefaultRule: option.RawDefaultRule{IPCIDR: direct},
		RuleAction:     toDirect,
	}})
	return append(out, match(s.Except, toExcept)...)
}

func TunInbound(s domain.Settings, resolverIPs []netip.Prefix) option.Inbound {
	var address, routes []netip.Prefix
	if s.IPv4() {
		address = append(address, netip.MustParsePrefix("172.19.0.1/30"))
		routes = append(routes, netip.MustParsePrefix("0.0.0.0/0"))
	}
	if s.IPv6() {
		address = append(address, netip.MustParsePrefix("fdfe:dcba:9876::1/126"))
		routes = append(routes, netip.MustParsePrefix("::/0"))
	}

	tunOpts := &option.TunInboundOptions{
		InterfaceName: domain.TunInterface,
		MTU:           uint32(s.TunMTU),
		Stack:         s.TunStack,
		Address:       address,
		AutoRoute:     true,
		StrictRoute:   s.TunStrict == "on",
		RouteAddress:  append(routes, resolverIPs...),
	}
	return option.Inbound{Type: C.TypeTun, Tag: "tun-in", Options: tunOpts}
}

func resolved(n domain.Node, s domain.Settings) (domain.Node, error) {
	if _, err := netip.ParseAddr(n.Server); err == nil {
		return n, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupNetIP(ctx, network(s), n.Server)
	if err != nil {
		return n, fmt.Errorf("could not resolve %s: %w", n.Server, err)
	}
	if len(ips) == 0 {
		return n, fmt.Errorf("no addresses for %s", n.Server)
	}
	switch {
	case n.TLS != nil && n.TLS.SNI == "":
		tls := *n.TLS
		tls.SNI = n.Server
		n.TLS = &tls
	case n.TLS == nil && tlsOnly(n.Protocol):
		n.TLS = &domain.TLS{SNI: n.Server}
	}
	n.Server = ips[0].Unmap().String()
	return n, nil
}

var dnsStrategy = map[string]option.DomainStrategy{
	"ipv4": option.DomainStrategy(C.DomainStrategyIPv4Only),
	"ipv6": option.DomainStrategy(C.DomainStrategyIPv6Only),
}

func network(s domain.Settings) string {
	switch s.IPVersion {
	case "ipv4":
		return "ip4"
	case "ipv6":
		return "ip6"
	}
	return "ip"
}

func ProbeTag(i int) string { return "p" + strconv.Itoa(i) }

func ProbeConfig(nodes []domain.Node, s domain.Settings, logPath string) *option.Options {
	opts := &option.Options{
		Log:   &option.LogOptions{Level: s.LogLevel, Output: logPath},
		Route: &option.RouteOptions{AutoDetectInterface: true},
	}
	resolvedNodes := make([]domain.Node, len(nodes))
	var wg sync.WaitGroup
	for i, n := range nodes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r, err := resolved(n, s); err == nil {
				n = r
			}
			resolvedNodes[i] = n
		}()
	}
	wg.Wait()

	for i, n := range resolvedNodes {
		_ = add(opts, n, ProbeTag(i))
	}
	return opts
}

func add(opts *option.Options, n domain.Node, tag string) error {
	if n.Protocol == domain.WG {
		ep, err := wgEndpoint(n, tag)
		if err != nil {
			return err
		}
		opts.Endpoints = append(opts.Endpoints, *ep)
		return nil
	}

	out, err := newOutbound(n, tag)
	if err != nil {
		return err
	}
	if ss, ok := out.Options.(*option.ShadowsocksOutboundOptions); ok && n.ShadowTLS != nil {
		ss.Detour = tag + "-stls"
		opts.Outbounds = append(opts.Outbounds, option.Outbound{
			Type: C.TypeShadowTLS, Tag: ss.Detour,
			Options: &option.ShadowTLSOutboundOptions{
				ServerOptions: server(n),
				Version:       n.ShadowTLS.Version,
				Password:      n.ShadowTLS.Password,
				OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
					TLS: &option.OutboundTLSOptions{Enabled: true, ServerName: n.ShadowTLS.SNI},
				},
			},
		})
	}
	opts.Outbounds = append(opts.Outbounds, *out)
	return nil
}

func wgEndpoint(n domain.Node, tag string) (*option.Endpoint, error) {
	w := n.WireGuard
	if w == nil || w.PrivateKey == "" || len(w.Address) == 0 {
		return nil, fmt.Errorf("wireguard: no key material")
	}
	address := make([]netip.Prefix, 0, len(w.Address))
	for _, s := range w.Address {
		p, err := netip.ParsePrefix(s)
		if err != nil {
			return nil, fmt.Errorf("wireguard: %w", err)
		}
		address = append(address, p)
	}
	return &option.Endpoint{Type: C.TypeWireGuard, Tag: tag, Options: &option.WireGuardEndpointOptions{
		Address:    address,
		PrivateKey: w.PrivateKey,
		MTU:        w.MTU,
		Peers: []option.WireGuardPeer{{
			Address:                     n.Server,
			Port:                        uint16(n.Port),
			PublicKey:                   w.PeerPublicKey,
			PreSharedKey:                w.PreSharedKey,
			AllowedIPs:                  []netip.Prefix{netip.MustParsePrefix("0.0.0.0/0"), netip.MustParsePrefix("::/0")},
			Reserved:                    w.Reserved,
			PersistentKeepaliveInterval: 25,
		}},
	}}, nil
}

func newOutbound(n domain.Node, tag string) (*option.Outbound, error) {
	tls := buildTLS(n)
	if tls == nil && tlsOnly(n.Protocol) {
		tls = &option.OutboundTLSOptions{Enabled: true}
	}

	switch n.Protocol {
	case domain.VLess:
		pe := packetEncoding(n)
		return &option.Outbound{Type: C.TypeVLESS, Tag: tag, Options: &option.VLESSOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Flow:                        n.Auth.Flow,
			PacketEncoding:              &pe,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case domain.VMess:
		return &option.Outbound{Type: C.TypeVMess, Tag: tag, Options: &option.VMessOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Security:                    cmp.Or(n.Auth.Method, "auto"),
			AlterId:                     n.Auth.AlterID,
			PacketEncoding:              packetEncoding(n),
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case domain.Trojan:
		return &option.Outbound{Type: C.TypeTrojan, Tag: tag, Options: &option.TrojanOutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case domain.SS:
		return &option.Outbound{Type: C.TypeShadowsocks, Tag: tag, Options: &option.ShadowsocksOutboundOptions{
			ServerOptions: server(n),
			Method:        n.Auth.Method,
			Password:      n.Auth.Password,
		}}, nil

	case domain.HY2:
		var obfs *option.Hysteria2Obfs
		if n.Obfs != "" {
			obfs = &option.Hysteria2Obfs{Type: n.Obfs, Password: n.ObfsPassword}
		}
		return &option.Outbound{Type: C.TypeHysteria2, Tag: tag, Options: &option.Hysteria2OutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			Obfs:                        obfs,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case domain.HY1:
		return &option.Outbound{Type: C.TypeHysteria, Tag: tag, Options: &option.HysteriaOutboundOptions{
			ServerOptions:               server(n),
			AuthString:                  n.Auth.Password,
			UpMbps:                      n.UpMbps,
			DownMbps:                    n.DownMbps,
			Obfs:                        n.ObfsPassword,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case domain.TUIC:
		return &option.Outbound{Type: C.TypeTUIC, Tag: tag, Options: &option.TUICOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Password:                    n.Auth.Password,
			CongestionControl:           n.Congestion,
			UDPRelayMode:                n.UDPRelayMode,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case domain.AnyTLS:
		return &option.Outbound{Type: C.TypeAnyTLS, Tag: tag, Options: &option.AnyTLSOutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case domain.SOCKS:
		return &option.Outbound{Type: C.TypeSOCKS, Tag: tag, Options: &option.SOCKSOutboundOptions{
			ServerOptions: server(n),
			Version:       "5",
			Username:      n.Auth.Username,
			Password:      n.Auth.Password,
		}}, nil

	case domain.HTTP:
		return &option.Outbound{Type: C.TypeHTTP, Tag: tag, Options: &option.HTTPOutboundOptions{
			ServerOptions:               server(n),
			Username:                    n.Auth.Username,
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil
	}
	return nil, fmt.Errorf("unsupported protocol %q", n.Protocol)
}

func server(n domain.Node) option.ServerOptions {
	return option.ServerOptions{Server: n.Server, ServerPort: uint16(n.Port)}
}

func addr(s string) *badoption.Addr {
	a := badoption.Addr(netip.MustParseAddr(s))
	return &a
}

func tlsOnly(p domain.Proto) bool {
	switch p {
	case domain.HY1, domain.HY2, domain.TUIC, domain.AnyTLS:
		return true
	}
	return false
}

// nil when plain tcp with no TLS/reality is enough
func buildTLS(n domain.Node) *option.OutboundTLSOptions {
	switch {
	case n.Reality != nil:
		tls := &option.OutboundTLSOptions{
			Enabled: true,
			Reality: &option.OutboundRealityOptions{
				Enabled:   true,
				PublicKey: n.Reality.PublicKey,
				ShortID:   n.Reality.ShortID,
			},
			UTLS: &option.OutboundUTLSOptions{Enabled: true, Fingerprint: "chrome"},
		}
		if n.TLS != nil {
			tls.ServerName = n.TLS.SNI
			tls.Insecure = n.TLS.Insecure
			tls.ALPN = n.TLS.ALPN
			if n.TLS.Fingerprint != "" {
				tls.UTLS.Fingerprint = n.TLS.Fingerprint
			}
		}
		return tls

	case n.TLS != nil:
		tls := &option.OutboundTLSOptions{
			Enabled:    true,
			ServerName: n.TLS.SNI,
			Insecure:   n.TLS.Insecure,
			ALPN:       n.TLS.ALPN,
		}
		if n.TLS.Fingerprint != "" {
			tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: n.TLS.Fingerprint}
		}
		return tls
	}
	return nil
}

// nil when the default (raw tcp) is enough
func buildTransport(n domain.Node) *option.V2RayTransportOptions {
	switch n.Transport.Network {
	case "ws":
		ws := option.V2RayWebsocketOptions{Path: n.Transport.Path}
		if n.Transport.Host != "" {
			ws.Headers = badoption.HTTPHeader{"Host": {n.Transport.Host}}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeWebsocket, WebsocketOptions: ws}
	case "grpc":
		return &option.V2RayTransportOptions{
			Type:        C.V2RayTransportTypeGRPC,
			GRPCOptions: option.V2RayGRPCOptions{ServiceName: n.Transport.ServiceName},
		}
	case "http":
		h := option.V2RayHTTPOptions{Path: n.Transport.Path}
		if n.Transport.Host != "" {
			h.Host = badoption.Listable[string]{n.Transport.Host}
		}
		return &option.V2RayTransportOptions{Type: C.V2RayTransportTypeHTTP, HTTPOptions: h}
	case "httpupgrade":
		return &option.V2RayTransportOptions{
			Type: C.V2RayTransportTypeHTTPUpgrade,
			HTTPUpgradeOptions: option.V2RayHTTPUpgradeOptions{
				Path: n.Transport.Path,
				Host: n.Transport.Host,
			},
		}
	}
	return nil
}

// final is the outbound nothing claimed
func final(s domain.Settings) string {
	if s.Mode == domain.DirectAll {
		return "direct"
	}
	return Tag
}
