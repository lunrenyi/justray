package domain

import (
	"fmt"
	"net/netip"
	"net/url"
	"reflect"
	"slices"
	"strings"
)

const (
	DefaultPort     = 10808
	DefaultDNS      = "1.1.1.1"
	DefaultLogLevel = "error"
	DefaultTunMTU   = 9000
	DefaultTunStack = "gvisor"
	DefaultProbeURL = "http://connectivitycheck.gstatic.com/generate_204"
	TunInterface    = "justray"
)

const (
	ProxyAll  = "proxy all"
	DirectAll = "direct all"
)

var (
	LogLevels  = []string{"error", "warn", "info", "debug"}
	TunStacks  = []string{"gvisor", "system", "mixed"}
	IPVersions = []string{"auto", "ipv4", "ipv6"}
	Modes      = []string{ProxyAll, DirectAll}
	Toggle     = []string{"on", "off"}
)

type Settings struct {
	Port         int    `yaml:"port,omitempty"`
	LogLevel     string `yaml:"log_level,omitempty"`
	ProbeURL     string `yaml:"probe_url,omitempty"`
	RefreshEvery int    `yaml:"refresh_hours,omitempty"` // 0 = never
	KillSwitch   string `yaml:"kill_switch,omitempty"`   // on/off, empty = off
	Autostart    string `yaml:"-"`                       // on/off, kept by the OS, not this file

	Mode    string   `yaml:"mode,omitempty"` // proxy-all/direct-all, empty = proxy-all
	Except  []string `yaml:"except,omitempty"`
	Blocked []string `yaml:"blocked,omitempty"`

	DNS         string `yaml:"dns,omitempty"`
	DNSHijack   string `yaml:"dns_hijack,omitempty"` // on/off, empty = on
	IPVersion   string `yaml:"ip_version,omitempty"`
	BypassLocal string `yaml:"bypass_local,omitempty"` // on/off, empty = on
	TunMTU      int    `yaml:"tun_mtu,omitempty"`
	TunStack    string `yaml:"tun_stack,omitempty"`
	TunStrict   string `yaml:"tun_strict_route,omitempty"` // on/off, empty = on
	BlockQUIC   string `yaml:"block_quic,omitempty"`       // on/off, empty = on
}

func (s Settings) IPv4() bool { return s.IPVersion != "ipv6" }
func (s Settings) IPv6() bool { return s.IPVersion != "ipv4" }

// Equal reports whether two settings are identical
func (s Settings) Equal(o Settings) bool { return reflect.DeepEqual(s, o) }

// Normalize fills defaults and validates
func (s Settings) Normalize() (Settings, error) {
	if s.Port == 0 {
		s.Port = DefaultPort
	}
	if s.Port < 1 || s.Port > 65535 {
		return s, fmt.Errorf("port %d is out of range", s.Port)
	}
	if s.DNS = strings.TrimSpace(s.DNS); s.DNS == "" {
		s.DNS = DefaultDNS
	}
	if _, err := netip.ParseAddr(s.DNS); err != nil {
		return s, fmt.Errorf("dns %q is not an ip address", s.DNS)
	}
	if s.LogLevel == "" {
		s.LogLevel = DefaultLogLevel
	}
	if !slices.Contains(LogLevels, s.LogLevel) {
		return s, fmt.Errorf("log level %q is not one of %s", s.LogLevel, strings.Join(LogLevels, ", "))
	}
	if s.TunMTU == 0 {
		s.TunMTU = DefaultTunMTU
	}
	if s.TunMTU < 576 || s.TunMTU > 65535 {
		return s, fmt.Errorf("mtu %d is out of range", s.TunMTU)
	}
	if s.TunStack == "" {
		s.TunStack = DefaultTunStack
	}
	if !slices.Contains(TunStacks, s.TunStack) {
		return s, fmt.Errorf("stack %q is not one of %s", s.TunStack, strings.Join(TunStacks, ", "))
	}

	if s.ProbeURL = strings.TrimSpace(s.ProbeURL); s.ProbeURL == "" {
		s.ProbeURL = DefaultProbeURL
	}
	if u, err := url.Parse(s.ProbeURL); err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return s, fmt.Errorf("probe url %q is not an http url", s.ProbeURL)
	}
	if s.IPVersion == "" {
		s.IPVersion = "auto"
	}
	if !slices.Contains(IPVersions, s.IPVersion) {
		return s, fmt.Errorf("ip version %q is not one of %s", s.IPVersion, strings.Join(IPVersions, ", "))
	}

	// a slice, not a map: the first reported error has to be stable
	for _, t := range []struct {
		name string
		v    *string
		def  string
	}{
		{"strict route", &s.TunStrict, "on"},
		{"dns hijack", &s.DNSHijack, "on"},
		{"block quic", &s.BlockQUIC, "on"},
		{"local networks", &s.BypassLocal, "on"},
		{"autostart", &s.Autostart, "off"},
		{"kill switch", &s.KillSwitch, "off"},
	} {
		if *t.v == "" {
			*t.v = t.def
		}
		if !slices.Contains(Toggle, *t.v) {
			return s, fmt.Errorf("%s must be on or off", t.name)
		}
	}
	if s.RefreshEvery < 0 || s.RefreshEvery > 24*30 {
		return s, fmt.Errorf("refresh interval %dh is out of range", s.RefreshEvery)
	}

	if s.Mode == "" {
		s.Mode = ProxyAll
	}
	if !slices.Contains(Modes, s.Mode) {
		return s, fmt.Errorf("mode %q is not one of %s", s.Mode, strings.Join(Modes, ", "))
	}

	for _, list := range []*[]string{&s.Except, &s.Blocked} {
		out := make([]string, 0, len(*list))
		for _, raw := range *list {
			rule, err := ParseRule(raw)
			if err != nil {
				return s, err
			}
			if !slices.Contains(out, rule) {
				out = append(out, rule)
			}
		}
		*list = out
	}
	return s, nil
}

// ParseRule canonicalises a network, domain or program rule
func ParseRule(raw string) (string, error) {
	rule := strings.TrimSpace(raw)
	if p, err := ParsePrefix(rule); err == nil {
		return p.String(), nil
	}
	rule = strings.Trim(strings.TrimPrefix(rule, "*."), ".")
	if rule == "" || strings.Contains(rule, "://") || strings.ContainsAny(rule, "\t\n\r@?#") {
		return "", fmt.Errorf("%q is not a network, a domain or a program", raw)
	}
	return rule, nil
}

// SplitRules splits entries into cidrs, domains, names and paths
func SplitRules(list []string) (cidrs, domains, names, paths []string) {
	for _, rule := range list {
		lower := strings.ToLower(rule)
		switch {
		case isPrefix(rule):
			cidrs = append(cidrs, rule)
		case strings.ContainsAny(rule, `/\`):
			paths = append(paths, rule)
		case strings.Contains(rule, " "), strings.HasSuffix(lower, ".exe"):
			names = append(names, rule)
		case strings.Contains(rule, "."):
			domains = append(domains, lower)
		default:
			names = append(names, rule)
			domains = append(domains, lower)
		}
	}
	return cidrs, domains, names, paths
}

func isPrefix(rule string) bool {
	_, err := netip.ParsePrefix(rule)
	return err == nil
}

// ParsePrefix accepts a CIDR or a bare address
func ParsePrefix(raw string) (netip.Prefix, error) {
	raw = strings.TrimSpace(raw)
	if p, err := netip.ParsePrefix(raw); err == nil {
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("%q is not an ip or a cidr", raw)
	}
	return netip.PrefixFrom(a, a.BitLen()), nil
}
