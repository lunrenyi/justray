package singbox

import (
	"net/netip"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"

	"github.com/luynrs/justray/internal/shared/domain"
)

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

// final is the outbound nothing claimed
func final(s domain.Settings) string {
	if s.Mode == domain.DirectAll {
		return "direct"
	}
	return Tag
}
