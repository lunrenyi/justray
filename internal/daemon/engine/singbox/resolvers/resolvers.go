package resolvers

import "net/netip"

func add(dst []netip.Prefix, seen map[netip.Addr]bool, a netip.Addr) []netip.Prefix {
	if !a.IsValid() || a.Zone() != "" || a.IsLoopback() || a.IsUnspecified() || seen[a] {
		return dst
	}
	seen[a] = true
	bits := 32
	if a.Is6() {
		bits = 128
	}
	return append(dst, netip.PrefixFrom(a.Unmap(), bits))
}
