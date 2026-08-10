package protocols

//
// Shadowsocks
//

import (
	"cmp"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/luynrs/justxray/internal/parser/proxy"
)

// ss://base64(method:password)@host:port#remark (SIP002) or the legacy
// ss://base64(method:password@host:port)#remark
func ParseSS(uri string) (proxy.Node, error) {
	rest := strings.TrimPrefix(uri, "ss://")

	rest, remark, _ := strings.Cut(rest, "#")
	if unescaped, err := url.PathUnescape(remark); err == nil {
		remark = unescaped
	}
	rest, _, _ = strings.Cut(rest, "?") // plugin= and friends, which xray can't do anyway

	var method, password, hp string
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		method, password = splitCreds(rest[:at])
		hp = rest[at+1:]
	} else {
		decoded, err := Unbase64(rest)
		if err != nil {
			return proxy.Node{}, fmt.Errorf("ss: base64: %w", err)
		}
		full := string(decoded)
		at := strings.LastIndexByte(full, '@')
		if at < 0 {
			return proxy.Node{}, fmt.Errorf("ss: missing host")
		}
		method, password, _ = strings.Cut(full[:at], ":")
		hp = full[at+1:]
	}
	if method == "" || password == "" {
		return proxy.Node{}, fmt.Errorf("ss: missing method/password")
	}

	host, p, err := net.SplitHostPort(strings.TrimSuffix(hp, "/")) // SIP002 allows an empty path
	if err != nil {
		return proxy.Node{}, fmt.Errorf("ss: %w", err)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("ss: bad port %q", p)
	}

	return proxy.Node{
		ID:        id(uri),
		Name:      cmp.Or(remark, host),
		Protocol:  proxy.SS,
		Server:    host,
		Port:      port,
		Auth:      proxy.Auth{Method: method, Password: password},
		Transport: proxy.Transport{Network: "tcp"},
	}, nil
}

// SIP002 credentials are b64
func splitCreds(blob string) (method, password string) {
	if decoded, err := Unbase64(blob); err == nil {
		blob = string(decoded)
	}
	method, password, _ = strings.Cut(blob, ":")
	return method, password
}
