package protocols

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/luynrs/justray/internal/domain"
)

// vless://uuid@host:port?...#remark
func ParseVLess(uri string) (domain.Node, error) {
	u, host, port, err := parseURL("vless", uri)
	if err != nil {
		return domain.Node{}, err
	}
	if u.User == nil || u.User.Username() == "" {
		return domain.Node{}, fmt.Errorf("vless: missing uuid")
	}

	q := u.Query()
	n := domain.Node{
		Name:           cmp.Or(u.Fragment, host),
		Protocol:       domain.VLess,
		Server:         host,
		Port:           port,
		Auth:           domain.Auth{UUID: u.User.Username(), Flow: q.Get("flow")},
		Transport:      transport(q),
		PacketEncoding: cmp.Or(q.Get("packetEncoding"), q.Get("packet_encoding")),
	}

	sec := strings.ToLower(q.Get("security"))
	if sec == "" && q.Get("pbk") != "" {
		sec = "reality"
	} else if sec == "" && (truthy(q.Get("tls")) || q.Get("sni") != "") {
		sec = "tls"
	}

	switch sec {
	case "reality":
		n.Reality = &domain.Reality{PublicKey: q.Get("pbk"), ShortID: q.Get("sid")}
		fallthrough
	case "tls", "xtls":
		n.TLS = tlsFrom(q, host)
	}
	return n, nil
}
