package protocols

// VLESS

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/luynrs/justray/internal/shared/domain"
)

// vless://uuid@host:port?...#remark
func ParseVLess(uri string) (domain.Node, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return domain.Node{}, fmt.Errorf("vless: %w", err)
	}
	if u.User == nil || u.User.Username() == "" {
		return domain.Node{}, fmt.Errorf("vless: missing uuid")
	}
	host, port, err := hostPort(u.Host)
	if err != nil {
		return domain.Node{}, fmt.Errorf("vless: %w", err)
	}

	q := u.Query()
	n := domain.Node{
		Name:           cmp.Or(u.Fragment, host),
		Protocol:       domain.VLess,
		Server:         host,
		Port:           port,
		Auth:           domain.Auth{UUID: u.User.Username(), Flow: q.Get("flow")},
		Transport:      transport(q),
		PacketEncoding: q.Get("packetEncoding"),
	}

	switch strings.ToLower(q.Get("security")) {
	case "reality":
		n.Reality = &domain.Reality{PublicKey: q.Get("pbk"), ShortID: q.Get("sid")}
		fallthrough
	case "tls", "xtls":
		n.TLS = &domain.TLS{
			SNI:         cmp.Or(q.Get("sni"), host),
			ALPN:        splitComma(q.Get("alpn")),
			Fingerprint: q.Get("fp"),
			Insecure:    insecureFlag(q),
		}
	}
	n.ID = nodeID(n)
	return n, nil
}
