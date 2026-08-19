package core

import (
	"cmp"
	"fmt"
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/luynrs/justray/internal/parser/proxy"
)

// tun is the interface name to bring up a tun inbound for, or "" for none
func Build(n proxy.Node, port int, logPath, tun string) (*option.Options, error) {
	out, err := Outbound(n, "proxy")
	if err != nil {
		return nil, err
	}

	inbounds := []option.Inbound{
		{Type: C.TypeMixed, Tag: "mixed-in", Options: &option.HTTPMixedInboundOptions{
			ListenOptions: option.ListenOptions{Listen: addr("127.0.0.1"), ListenPort: uint16(port)},
		}},
	}
	if tun != "" {
		inbounds = append(inbounds, option.Inbound{Type: C.TypeTun, Tag: "tun-in", Options: &option.TunInboundOptions{
			InterfaceName: tun,
			MTU:           1500,
			Address: []netip.Prefix{
				netip.MustParsePrefix("172.19.0.1/30"),
				netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
			},
			AutoRoute:   true,
			StrictRoute: true,
		}})
	}

	return &option.Options{
		Log: &option.LogOptions{Level: "error", Output: logPath},
		DNS: &option.DNSOptions{
			RawDNSOptions: option.RawDNSOptions{
				Servers: []option.DNSServerOptions{
					{Type: C.DNSTypeLocal, Tag: "local-dns", Options: &option.LocalDNSServerOptions{
						PreferGo: true,
					}},
				},
				Rules: []option.DNSRule{
					{DefaultOptions: option.DefaultDNSRule{
						RawDefaultDNSRule: option.RawDefaultDNSRule{
							Outbound: []string{"any"},
						},
						DNSRuleAction: option.DNSRuleAction{
							Action: "route",
							RouteOptions: option.DNSRouteActionOptions{
								Server: "local-dns",
							},
						},
					}},
				},
			},
		},
		Inbounds: inbounds,
		Outbounds: []option.Outbound{
			*out,
			{Type: C.TypeDirect, Tag: "direct", Options: &option.DirectOutboundOptions{}},
			{Type: C.TypeBlock, Tag: "block", Options: &option.StubOptions{}},
		},
		Route: &option.RouteOptions{
			Final:               "proxy",
			AutoDetectInterface: tun != "",
		},
	}, nil
}

// one local socks inbound per node, routed to that node's outbound
func ProbeConfig(nodes []proxy.Node, ports []int) (*option.Options, error) {
	var inbounds []option.Inbound
	var outbounds []option.Outbound
	var rules []option.Rule
	for i, n := range nodes {
		tag := fmt.Sprintf("p%d", i)
		out, err := Outbound(n, tag)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, option.Inbound{
			Type: C.TypeSOCKS, Tag: "in" + tag,
			Options: &option.SocksInboundOptions{
				ListenOptions: option.ListenOptions{Listen: addr("127.0.0.1"), ListenPort: uint16(ports[i])},
			},
		})
		outbounds = append(outbounds, *out)
		rules = append(rules, option.Rule{
			Type: C.RuleTypeDefault,
			DefaultOptions: option.DefaultRule{
				RawDefaultRule: option.RawDefaultRule{Inbound: []string{"in" + tag}},
				RuleAction: option.RuleAction{
					Action:       C.RuleActionTypeRoute,
					RouteOptions: option.RouteActionOptions{Outbound: tag},
				},
			},
		})
	}
	return &option.Options{
		Log:       &option.LogOptions{Level: "error"},
		Inbounds:  inbounds,
		Outbounds: outbounds,
		Route:     &option.RouteOptions{Rules: rules},
	}, nil
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
