package protocols

import (
	"cmp"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/luynrs/justray/internal/domain"
)

// wireguard://private-key@host:port?publickey=...&address=10.0.0.2/32#remark
func ParseWireGuard(uri string) (domain.Node, error) {
	u, host, port, err := parseURL("wireguard", encodeUserinfoSlash(uri))
	if err != nil {
		return domain.Node{}, err
	}
	q := u.Query()
	privateKey := ""
	if u.User != nil {
		privateKey = u.User.Username()
	}
	if privateKey == "" {
		privateKey = rawQuery(u, "privatekey")
	}
	if privateKey == "" {
		privateKey = q.Get("privatekey")
	}
	peerKey := rawQuery(u, "publickey")
	if peerKey == "" {
		peerKey = q.Get("publickey")
	}
	if privateKey == "" || peerKey == "" {
		return domain.Node{}, fmt.Errorf("wireguard: missing private/public key")
	}

	address := splitComma(q.Get("address"))
	if len(address) == 0 {
		address = addresses(q.Get("ip"), q.Get("ipv6"))
	}
	if len(address) == 0 {
		return domain.Node{}, fmt.Errorf("wireguard: missing address")
	}

	return domain.Node{
		Name:     cmp.Or(u.Fragment, host),
		Protocol: domain.WG,
		Server:   host,
		Port:     port,
		WireGuard: &domain.WireGuard{
			PrivateKey:    privateKey,
			PeerPublicKey: peerKey,
			PreSharedKey:  rawQuery(u, "presharedkey"),
			Address:       address,
			Reserved:      parseReserved(rawQuery(u, "reserved")),
			MTU:           uint32(atoi(q.Get("mtu"))),
		},
	}, nil
}

func parseReserved(s string) []uint8 {
	if s == "" {
		return nil
	}
	s = strings.Trim(s, "[]")
	if strings.Contains(s, ",") {
		parts := strings.Split(s, ",")
		if len(parts) == 3 {
			res := make([]uint8, 3)
			for i, p := range parts {
				n, err := strconv.Atoi(strings.TrimSpace(p))
				if err != nil || n < 0 || n > 255 {
					return nil
				}
				res[i] = uint8(n)
			}
			return res
		}
	}
	if b, err := Unbase64(s); err == nil && len(b) == 3 {
		return b
	}
	return nil
}

func encodeUserinfoSlash(uri string) string {
	start := strings.Index(uri, "://")
	if start < 0 {
		return uri
	}
	start += 3
	end := strings.IndexByte(uri[start:], '@')
	if end < 0 {
		return uri
	}
	end += start
	if strings.ContainsAny(uri[start:end], "?#") {
		return uri
	}
	return uri[:start] + strings.ReplaceAll(uri[start:end], "/", "%2F") + uri[end:]
}

func rawQuery(u *url.URL, key string) string {
	for pair := range strings.SplitSeq(u.RawQuery, "&") {
		name, value, ok := strings.Cut(pair, "=")
		if ok && name == key {
			value, _ = url.PathUnescape(value)
			return value
		}
	}
	return ""
}
