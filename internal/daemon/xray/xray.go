package xray

import (
	"cmp"
	"encoding/json"
	"fmt"

	"github.com/luynrs/justxray/internal/parser/proxy"
)

var protocols = map[proxy.Proto]string{
	proxy.VLess:  "vless",
	proxy.VMess:  "vmess",
	proxy.Trojan: "trojan",
	proxy.SS:     "shadowsocks",
	proxy.HY2:    "hysteria",
}

func Build(n proxy.Node, socks, http int, logPath string) ([]byte, error) {
	proxy, err := Outbound(n, "proxy")
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(map[string]any{
		"log": map[string]any{"loglevel": "warning", "error": logPath},
		"inbounds": []any{
			map[string]any{
				"tag": "socks-in", "listen": "127.0.0.1", "port": socks, "protocol": "socks",
				"settings": map[string]any{"auth": "noauth", "udp": true},
			},
			map[string]any{"tag": "http-in", "listen": "127.0.0.1", "port": http, "protocol": "http"},
		},
		"outbounds": []any{
			proxy,
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{"domainStrategy": "IPIfNonMatch"},
	}, "", "  ")
}

func Outbound(n proxy.Node, tag string) (map[string]any, error) {
	protocol, ok := protocols[n.Protocol]
	if !ok {
		return nil, fmt.Errorf("unsupported protocol %q", n.Protocol)
	}
	settings, err := outbound(n)
	if err != nil {
		return nil, err
	}

	out := map[string]any{"tag": tag, "protocol": protocol, "settings": settings}
	if ss := stream(n); ss != nil {
		out["streamSettings"] = ss
	}
	return out, nil
}

// one local socks inbound per node, routed to that node's outbound
func ProbeConfig(nodes []proxy.Node, ports []int) ([]byte, error) {
	var inbounds, outbounds, rules []any
	for i, n := range nodes {
		tag := fmt.Sprintf("p%d", i)
		out, err := Outbound(n, tag)
		if err != nil {
			return nil, err
		}
		inbounds = append(inbounds, map[string]any{
			"tag": "in" + tag, "listen": "127.0.0.1", "port": ports[i], "protocol": "socks",
			"settings": map[string]any{"auth": "noauth", "udp": false},
		})
		outbounds = append(outbounds, out)
		rules = append(rules, map[string]any{
			"type": "field", "inboundTag": []any{"in" + tag}, "outboundTag": tag,
		})
	}
	return json.MarshalIndent(map[string]any{
		"log":       map[string]any{"loglevel": "error"},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   map[string]any{"rules": rules},
	}, "", "  ")
}

func outbound(n proxy.Node) (map[string]any, error) {
	switch n.Protocol {
	case proxy.VLess:
		user := map[string]any{"id": n.Auth.UUID, "encryption": "none"}
		if n.Auth.Flow != "" {
			user["flow"] = n.Auth.Flow
		}
		return vnext(n, user), nil

	case proxy.VMess:
		return vnext(n, map[string]any{
			"id":       n.Auth.UUID,
			"alterId":  n.Auth.AlterID,
			"security": cmp.Or(n.Auth.Method, "auto"),
		}), nil

	case proxy.Trojan:
		return servers(map[string]any{"address": n.Server, "port": n.Port, "password": n.Auth.Password}), nil

	case proxy.SS:
		return servers(map[string]any{
			"address": n.Server, "port": n.Port,
			"method": n.Auth.Method, "password": n.Auth.Password,
		}), nil

	case proxy.HY2:
		return map[string]any{"version": 2, "address": n.Server, "port": n.Port}, nil
	}
	return nil, fmt.Errorf("no outbound for %q", n.Protocol)
}

func vnext(n proxy.Node, user map[string]any) map[string]any {
	return map[string]any{"vnext": []any{
		map[string]any{"address": n.Server, "port": n.Port, "users": []any{user}},
	}}
}

func servers(s map[string]any) map[string]any {
	return map[string]any{"servers": []any{s}}
}

// nil when plain tcp with no tls is enough and xray's defaults will do
func stream(n proxy.Node) map[string]any {
	if n.Protocol == proxy.HY2 {
		return hysteriaStream(n)
	}

	network := cmp.Or(n.Transport.Network, "tcp")
	ss := map[string]any{"network": network}

	switch network {
	case "ws":
		ws := map[string]any{}
		if n.Transport.Path != "" {
			ws["path"] = n.Transport.Path
		}
		if n.Transport.Host != "" {
			ws["headers"] = map[string]string{"Host": n.Transport.Host}
		}
		ss["wsSettings"] = ws
	case "grpc":
		ss["grpcSettings"] = map[string]any{"serviceName": n.Transport.ServiceName}
	}

	switch {
	case n.Reality != nil:
		reality := map[string]any{"publicKey": n.Reality.PublicKey, "shortId": n.Reality.ShortID}
		if n.Reality.SpiderX != "" {
			reality["spiderX"] = n.Reality.SpiderX
		}
		if n.TLS != nil {
			reality["serverName"] = n.TLS.SNI
			if n.TLS.Fingerprint != "" {
				reality["fingerprint"] = n.TLS.Fingerprint
			}
		}
		ss["security"], ss["realitySettings"] = "reality", reality

	case n.TLS != nil:
		tls := map[string]any{"serverName": n.TLS.SNI, "allowInsecure": n.TLS.Insecure}
		if len(n.TLS.ALPN) > 0 {
			tls["alpn"] = n.TLS.ALPN
		}
		if n.TLS.Fingerprint != "" {
			tls["fingerprint"] = n.TLS.Fingerprint
		}
		ss["security"], ss["tlsSettings"] = "tls", tls

	case network == "tcp":
		return nil
	}
	return ss
}

func hysteriaStream(n proxy.Node) map[string]any {
	tls := map[string]any{"allowInsecure": false}
	if n.TLS != nil {
		tls["serverName"] = n.TLS.SNI
		tls["allowInsecure"] = n.TLS.Insecure
	}
	ss := map[string]any{
		"network":          "hysteria",
		"security":         "tls",
		"hysteriaSettings": map[string]any{"version": 2, "auth": n.Auth.Password},
		"tlsSettings":      tls,
	}
	if n.Obfs != "" {
		ss["finalmask"] = map[string]any{"udp": []any{
			map[string]any{"type": n.Obfs, "settings": map[string]any{"password": n.ObfsPassword}},
		}}
	}
	return ss
}
