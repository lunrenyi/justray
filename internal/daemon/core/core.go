package core

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/luynrs/justray/internal/parser/proxy"
)

func Build(n proxy.Node, port int, logPath, tun string) (*option.Options, error) {
	n, err := resolved(n)
	if err != nil {
		return nil, err
	}

	opts := &option.Options{
		Log: &option.LogOptions{Level: LogLevel(), Output: logPath},
		Inbounds: []option.Inbound{
			{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{
				ListenOptions: option.ListenOptions{Listen: addr("127.0.0.1"), ListenPort: uint16(port)},
			}},
		},
		Route: &option.RouteOptions{Final: "proxy"},
	}
	if err := Add(opts, n, "proxy"); err != nil {
		return nil, err
	}
	if tun == "" {
		return opts, nil
	}

	resolverIPs := resolvers()
	opts.Inbounds = append(opts.Inbounds, option.Inbound{Type: C.TypeTun, Tag: "tun-in", Options: &option.TunInboundOptions{
		InterfaceName: tun,
		MTU:           1500,
		Stack:         "gvisor",
		Address: []netip.Prefix{
			netip.MustParsePrefix("172.19.0.1/30"),
			netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
		},
		AutoRoute:   true,
		StrictRoute: true,
		RouteAddress: append([]netip.Prefix{
			netip.MustParsePrefix("0.0.0.0/0"),
			netip.MustParsePrefix("::/0"),
		}, resolverIPs...),
	}})
	opts.DNS = &option.DNSOptions{RawDNSOptions: option.RawDNSOptions{Servers: []option.DNSServerOptions{
		{Type: C.DNSTypeUDP, Tag: "remote", Options: &option.RemoteDNSServerOptions{
			RawLocalDNSServerOptions: option.RawLocalDNSServerOptions{
				DialerOptions: option.DialerOptions{Detour: "proxy"},
			},
			DNSServerAddressOptions: option.DNSServerAddressOptions{Server: cmp.Or(os.Getenv("JUSTRAY_DNS"), "1.1.1.1")},
		}},
	}}}
	opts.Route.AutoDetectInterface = true
	opts.Outbounds = append(opts.Outbounds, option.Outbound{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}})
	resolverCIDRs := make([]string, len(resolverIPs))
	for i, p := range resolverIPs {
		resolverCIDRs[i] = p.String()
	}
	opts.Route.Rules = []option.Rule{
		{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{Port: []uint16{53}},
			RuleAction:     option.RuleAction{Action: C.RuleActionTypeHijackDNS},
		}},
		{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
			RawDefaultRule: option.RawDefaultRule{IPCIDR: resolverCIDRs},
			RuleAction:     option.RuleAction{Action: C.RuleActionTypeRoute, RouteOptions: option.RouteActionOptions{Outbound: "direct"}},
		}},
	}
	return opts, nil
}

func LogLevel() string { return cmp.Or(os.Getenv("JUSTRAY_LOG"), "error") }

func resolved(n proxy.Node) (proxy.Node, error) {
	if _, err := netip.ParseAddr(n.Server); err == nil {
		return n, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", n.Server)
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
		n.TLS = &proxy.TLS{SNI: n.Server}
	}
	n.Server = ips[0].Unmap().String()
	return n, nil
}

func resolvers() []netip.Prefix {
	b, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var out []netip.Prefix
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != "nameserver" {
			continue
		}
		a, err := netip.ParseAddr(f[1])
		if err != nil || a.IsLoopback() {
			continue
		}
		bits := 32
		if a.Is6() {
			bits = 128
		}
		out = append(out, netip.PrefixFrom(a, bits))
	}
	return out
}

func ProbeTag(i int) string { return "p" + strconv.Itoa(i) }

func ProbeConfig(nodes []proxy.Node, logPath string) *option.Options {
	opts := &option.Options{
		Log:   &option.LogOptions{Level: LogLevel(), Output: logPath},
		Route: &option.RouteOptions{AutoDetectInterface: true},
	}
	for i, n := range nodes {
		_ = Add(opts, n, ProbeTag(i))
	}
	return opts
}

func Add(opts *option.Options, n proxy.Node, tag string) error {
	if n.Protocol == proxy.WG {
		ep, err := wgEndpoint(n, tag)
		if err != nil {
			return err
		}
		opts.Endpoints = append(opts.Endpoints, *ep)
		return nil
	}

	out, err := Outbound(n, tag)
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

func wgEndpoint(n proxy.Node, tag string) (*option.Endpoint, error) {
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

func Outbound(n proxy.Node, tag string) (*option.Outbound, error) {
	tls := buildTLS(n)
	if tls == nil && tlsOnly(n.Protocol) {
		tls = &option.OutboundTLSOptions{Enabled: true}
	}

	switch n.Protocol {
	case proxy.VLess:
		return &option.Outbound{Type: C.TypeVLESS, Tag: tag, Options: &option.VLESSOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Flow:                        n.Auth.Flow,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.VMess:
		return &option.Outbound{Type: C.TypeVMess, Tag: tag, Options: &option.VMessOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Security:                    cmp.Or(n.Auth.Method, "auto"),
			AlterId:                     n.Auth.AlterID,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.Trojan:
		return &option.Outbound{Type: C.TypeTrojan, Tag: tag, Options: &option.TrojanOutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.SS:
		return &option.Outbound{Type: C.TypeShadowsocks, Tag: tag, Options: &option.ShadowsocksOutboundOptions{
			ServerOptions: server(n),
			Method:        n.Auth.Method,
			Password:      n.Auth.Password,
		}}, nil

	case proxy.HY2:
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

	case proxy.HY1:
		return &option.Outbound{Type: C.TypeHysteria, Tag: tag, Options: &option.HysteriaOutboundOptions{
			ServerOptions:               server(n),
			AuthString:                  n.Auth.Password,
			UpMbps:                      n.UpMbps,
			DownMbps:                    n.DownMbps,
			Obfs:                        n.ObfsPassword,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case proxy.TUIC:
		return &option.Outbound{Type: C.TypeTUIC, Tag: tag, Options: &option.TUICOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Password:                    n.Auth.Password,
			CongestionControl:           n.Congestion,
			UDPRelayMode:                n.UDPRelayMode,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case proxy.AnyTLS:
		return &option.Outbound{Type: C.TypeAnyTLS, Tag: tag, Options: &option.AnyTLSOutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil

	case proxy.SOCKS:
		return &option.Outbound{Type: C.TypeSOCKS, Tag: tag, Options: &option.SOCKSOutboundOptions{
			ServerOptions: server(n),
			Version:       "5",
			Username:      n.Auth.Username,
			Password:      n.Auth.Password,
		}}, nil

	case proxy.HTTP:
		return &option.Outbound{Type: C.TypeHTTP, Tag: tag, Options: &option.HTTPOutboundOptions{
			ServerOptions:               server(n),
			Username:                    n.Auth.Username,
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: tls},
		}}, nil
	}
	return nil, fmt.Errorf("unsupported protocol %q", n.Protocol)
}

func server(n proxy.Node) option.ServerOptions {
	return option.ServerOptions{Server: n.Server, ServerPort: uint16(n.Port)}
}

func addr(s string) *badoption.Addr {
	a := badoption.Addr(netip.MustParseAddr(s))
	return &a
}

func tlsOnly(p proxy.Proto) bool {
	switch p {
	case proxy.HY1, proxy.HY2, proxy.TUIC, proxy.AnyTLS:
		return true
	}
	return false
}

// nil when plain tcp with no TLS/reality is enough
func buildTLS(n proxy.Node) *option.OutboundTLSOptions {
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
func buildTransport(n proxy.Node) *option.V2RayTransportOptions {
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
