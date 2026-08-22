package protocols

//
// Shadowsocks
//

import (
	"cmp"
	"fmt"
	"net/url"
	"strings"

	"github.com/luynrs/justray/internal/shared/domain"
)

// ss://base64(method:password)@host:port#remark (SIP002) or the legacy
// ss://base64(method:password@host:port)#remark
func ParseSS(uri string) (domain.Node, error) {
	rest := strings.TrimPrefix(uri, "ss://")

	rest, remark, _ := strings.Cut(rest, "#")
	if unescaped, err := url.PathUnescape(remark); err == nil {
		remark = unescaped
	}
	rest, query, hasQuery := strings.Cut(rest, "?")
	if hasQuery {
		qv, _ := url.ParseQuery(query)
		if err := checkPlugin(qv.Get("plugin")); err != nil {
			return domain.Node{}, fmt.Errorf("ss: %w", err)
		}
	}

	var method, password, hp string
	if at := strings.LastIndexByte(rest, '@'); at >= 0 {
		userinfo := rest[:at]
		if unescaped, err := url.PathUnescape(userinfo); err == nil {
			userinfo = unescaped
		}
		method, password = splitCreds(userinfo)
		hp = rest[at+1:]
	} else {
		decoded, err := Unbase64(rest)
		if err != nil {
			return domain.Node{}, fmt.Errorf("ss: base64: %w", err)
		}
		full := string(decoded)
		at := strings.LastIndexByte(full, '@')
		if at < 0 {
			return domain.Node{}, fmt.Errorf("ss: missing host")
		}
		method, password, _ = strings.Cut(full[:at], ":")
		hp = full[at+1:]
	}
	if method == "" || password == "" {
		return domain.Node{}, fmt.Errorf("ss: missing method/password")
	}

	host, port, err := hostPort(strings.TrimSuffix(hp, "/")) // SIP002 allows an empty path
	if err != nil {
		return domain.Node{}, fmt.Errorf("ss: %w", err)
	}

	n := domain.Node{
		Name:     cmp.Or(remark, host),
		Protocol: domain.SS,
		Server:   host,
		Port:     port,
		Auth:     domain.Auth{Method: method, Password: password},
	}
	n.ID = nodeID(n)
	return n, nil
}

// SIP002 credentials are b64
func splitCreds(blob string) (method, password string) {
	if strings.Contains(blob, ":") {
		method, password, _ = strings.Cut(blob, ":")
		return method, password
	}
	if decoded, err := Unbase64(blob); err == nil {
		blob = string(decoded)
	}
	method, password, _ = strings.Cut(blob, ":")
	return method, password
}
