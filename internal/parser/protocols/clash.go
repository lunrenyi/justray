package protocols

import (
	"cmp"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/luynrs/justray/internal/parser/proxy"
)

type clashProxy struct {
	Name              string   `yaml:"name"`
	Type              string   `yaml:"type"`
	Server            string   `yaml:"server"`
	Port              int      `yaml:"port"`
	UUID              string   `yaml:"uuid"`
	Password          string   `yaml:"password"`
	Cipher            string   `yaml:"cipher"`
	AlterID           int      `yaml:"alterId"`
	Network           string   `yaml:"network"`
	TLS               bool     `yaml:"tls"`
	SkipCertVerify    bool     `yaml:"skip-cert-verify"`
	ServerName        string   `yaml:"servername"`
	SNI               string   `yaml:"sni"`
	Flow              string   `yaml:"flow"`
	ClientFingerprint string   `yaml:"client-fingerprint"`
	ALPN              []string `yaml:"alpn"`
	Obfs              string   `yaml:"obfs"`

	WSOpts *struct {
		Path    string            `yaml:"path"`
		Headers map[string]string `yaml:"headers"`
	} `yaml:"ws-opts"`
	GRPCOpts *struct {
		ServiceName string `yaml:"grpc-service-name"`
	} `yaml:"grpc-opts"`
	RealityOpts *struct {
		PublicKey string `yaml:"public-key"`
		ShortID   string `yaml:"short-id"`
	} `yaml:"reality-opts"`
}

// Clash/Mihomo "proxies:" list
func ParseClash(raw []byte) ([]proxy.Node, error) {
	var doc struct {
		Proxies []clashProxy `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("clash: %w", err)
	}

	var nodes []proxy.Node
	for _, p := range doc.Proxies {
		if n, err := clashNode(p); err == nil {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("clash: no supported proxies")
	}
	return nodes, nil
}

func clashNode(p clashProxy) (proxy.Node, error) {
	if p.Server == "" || p.Port == 0 {
		return proxy.Node{}, fmt.Errorf("clash: missing server/port")
	}
	n := proxy.Node{
		ID:     id(fmt.Sprintf("clash:%s:%s:%d:%s:%s", p.Type, p.Server, p.Port, p.UUID, p.Password)),
		Name:   cmp.Or(p.Name, p.Server),
		Server: p.Server,
		Port:   p.Port,
	}
	tls := &proxy.TLS{
		SNI:         cmp.Or(p.SNI, p.ServerName, p.Server),
		ALPN:        p.ALPN,
		Fingerprint: p.ClientFingerprint,
		Insecure:    p.SkipCertVerify,
	}

	switch strings.ToLower(p.Type) {
	case "vless":
		if p.UUID == "" {
			return proxy.Node{}, fmt.Errorf("clash: vless missing uuid")
		}
		n.Protocol = proxy.VLess
		n.Auth = proxy.Auth{UUID: p.UUID, Flow: p.Flow}
		n.Transport = clashTransport(p)
		if p.TLS || p.RealityOpts != nil {
			n.TLS = tls
		}
		if p.RealityOpts != nil {
			n.Reality = &proxy.Reality{PublicKey: p.RealityOpts.PublicKey, ShortID: p.RealityOpts.ShortID}
		}

	case "vmess":
		if p.UUID == "" {
			return proxy.Node{}, fmt.Errorf("clash: vmess missing uuid")
		}
		n.Protocol = proxy.VMess
		n.Auth = proxy.Auth{UUID: p.UUID, AlterID: p.AlterID, Method: strings.ToLower(cmp.Or(p.Cipher, "auto"))}
		n.Transport = clashTransport(p)
		if p.TLS {
			n.TLS = tls
		}

	case "trojan":
		if p.Password == "" {
			return proxy.Node{}, fmt.Errorf("clash: trojan missing password")
		}
		n.Protocol = proxy.Trojan
		n.Auth = proxy.Auth{Password: p.Password}
		n.Transport = clashTransport(p)
		n.TLS = tls

	case "ss", "shadowsocks":
		if p.Cipher == "" || p.Password == "" {
			return proxy.Node{}, fmt.Errorf("clash: ss missing cipher/password")
		}
		n.Protocol = proxy.SS
		n.Auth = proxy.Auth{Method: p.Cipher, Password: p.Password}
		n.Transport = proxy.Transport{Network: "tcp"}

	case "hysteria2", "hy2":
		if p.Password == "" {
			return proxy.Node{}, fmt.Errorf("clash: hysteria2 missing password")
		}
		n.Protocol = proxy.HY2
		n.Auth = proxy.Auth{Password: p.Password}
		n.Transport = proxy.Transport{Network: "quic"}
		n.TLS = tls
		n.Obfs = p.Obfs

	default:
		return proxy.Node{}, fmt.Errorf("clash: unsupported type %q", p.Type)
	}
	return n, nil
}

func clashTransport(p clashProxy) proxy.Transport {
	t := proxy.Transport{Network: strings.ToLower(cmp.Or(p.Network, "tcp"))}
	switch t.Network {
	case "ws":
		if p.WSOpts != nil {
			t.Path = p.WSOpts.Path
			for k, v := range p.WSOpts.Headers {
				if strings.EqualFold(k, "Host") {
					t.Host = v
				}
			}
		}
	case "grpc":
		if p.GRPCOpts != nil {
			t.Path, t.ServiceName = p.GRPCOpts.ServiceName, p.GRPCOpts.ServiceName
		}
	}
	return t
}
