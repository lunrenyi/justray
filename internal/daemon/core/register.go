package core

import (
	"context"

	sbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	boxservice "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/protocol/block"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/http"
	"github.com/sagernet/sing-box/protocol/hysteria2"
	"github.com/sagernet/sing-box/protocol/shadowsocks"
	"github.com/sagernet/sing-box/protocol/socks"
	"github.com/sagernet/sing-box/protocol/trojan"
	"github.com/sagernet/sing-box/protocol/tun"
	"github.com/sagernet/sing-box/protocol/vless"
	"github.com/sagernet/sing-box/protocol/vmess"
)

// only the protocols justray actually speaks, not the vendor's full include
// package (which also drags in tor, ssh, tailscale, caddy/ACME, cronet...)
func Context(ctx context.Context) context.Context {
	return sbox.Context(ctx, inboundRegistry(), outboundRegistry(), endpoint.NewRegistry(), dnsTransportRegistry(), boxservice.NewRegistry())
}

func inboundRegistry() *inbound.Registry {
	registry := inbound.NewRegistry()
	socks.RegisterInbound(registry)
	http.RegisterInbound(registry)
	tun.RegisterInbound(registry)
	return registry
}

func outboundRegistry() *outbound.Registry {
	registry := outbound.NewRegistry()
	direct.RegisterOutbound(registry)
	block.RegisterOutbound(registry)
	vless.RegisterOutbound(registry)
	vmess.RegisterOutbound(registry)
	trojan.RegisterOutbound(registry)
	shadowsocks.RegisterOutbound(registry)
	hysteria2.RegisterOutbound(registry)
	return registry
}

// "local" is the fallback resolver reached for whenever an
// outbound needs to resolve a server hostname
func dnsTransportRegistry() *dns.TransportRegistry {
	registry := dns.NewTransportRegistry()
	local.RegisterTransport(registry)
	return registry
}
