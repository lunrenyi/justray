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
	out, err := Outbound(n, "proxy")
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
		Outbounds: []option.Outbound{*out},
		Route:     &option.RouteOptions{Final: "proxy"},
	}
	if tun == "" {
		return opts, nil
	}

	opts.Inbounds = append(opts.Inbounds, option.Inbound{Type: C.TypeTun, Tag: "tun-in", Options: &option.TunInboundOptions{
		InterfaceName: tun,
		MTU:           1500,
		Stack:         "gvisor",
		Address: []netip.Prefix{
			netip.MustParsePrefix("172.19.0.1/30"),
		},
		AutoRoute:    true,
		StrictRoute:  true,
		RouteAddress: append([]netip.Prefix{netip.MustParsePrefix("0.0.0.0/0")}, resolvers()...),
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
	opts.Route.Rules = []option.Rule{{Type: C.RuleTypeDefault, DefaultOptions: option.DefaultRule{
		RawDefaultRule: option.RawDefaultRule{Port: []uint16{53}},
		RuleAction:     option.RuleAction{Action: C.RuleActionTypeHijackDNS},
	}}}
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
	if err != nil || len(ips) == 0 {
		return n, fmt.Errorf("could not resolve %s: %w", n.Server, err)
	}
	switch {
	case n.TLS != nil && n.TLS.SNI == "":
		tls := *n.TLS
		tls.SNI = n.Server
		n.TLS = &tls
	case n.TLS == nil && n.Protocol == proxy.HY2:
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
		if err != nil || !a.Is4() || a.IsLoopback() {
			continue
		}
		out = append(out, netip.PrefixFrom(a, 32))
	}
	return out
}

func ProbeTag(i int) string { return "p" + strconv.Itoa(i) }

func ProbeConfig(nodes []proxy.Node, logPath string) *option.Options {
	var outbounds []option.Outbound
	for i, n := range nodes {
		if out, err := Outbound(n, ProbeTag(i)); err == nil {
			outbounds = append(outbounds, *out)
		}
	}
	return &option.Options{
		Log:       &option.LogOptions{Level: LogLevel(), Output: logPath},
		Outbounds: outbounds,
	}
}

func Outbound(n proxy.Node, tag string) (*option.Outbound, error) {
	switch n.Protocol {
	case proxy.VLess:
		return &option.Outbound{Type: C.TypeVLESS, Tag: tag, Options: &option.VLESSOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Flow:                        n.Auth.Flow,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: buildTLS(n)},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.VMess:
		return &option.Outbound{Type: C.TypeVMess, Tag: tag, Options: &option.VMessOutboundOptions{
			ServerOptions:               server(n),
			UUID:                        n.Auth.UUID,
			Security:                    cmp.Or(n.Auth.Method, "auto"),
			AlterId:                     n.Auth.AlterID,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: buildTLS(n)},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.Trojan:
		return &option.Outbound{Type: C.TypeTrojan, Tag: tag, Options: &option.TrojanOutboundOptions{
			ServerOptions:               server(n),
			Password:                    n.Auth.Password,
			OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{TLS: buildTLS(n)},
			Transport:                   buildTransport(n),
		}}, nil

	case proxy.SS:
		return &option.Outbound{Type: C.TypeShadowsocks, Tag: tag, Options: &option.ShadowsocksOutboundOptions{
			ServerOptions: server(n),
			Method:        n.Auth.Method,
			Password:      n.Auth.Password,
		}}, nil

	case proxy.HY2:
		tls := &option.OutboundTLSOptions{Enabled: true}
		if n.TLS != nil {
			tls.ServerName, tls.Insecure = n.TLS.SNI, n.TLS.Insecure
		}
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
		}
		if n.TLS != nil {
			tls.ServerName = n.TLS.SNI
			if n.TLS.Fingerprint != "" {
				tls.UTLS = &option.OutboundUTLSOptions{Enabled: true, Fingerprint: n.TLS.Fingerprint}
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
	}
	return nil
}
