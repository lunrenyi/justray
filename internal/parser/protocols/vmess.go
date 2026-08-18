package protocols

//
// VMESS
//

import (
	"cmp"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/luynrs/justray/internal/parser/proxy"
)

// v2rayn schema
type vmessLink struct {
	PS   string  `json:"ps"`
	Add  string  `json:"add"`
	Port flexInt `json:"port"`
	ID   string  `json:"id"`
	AID  flexInt `json:"aid"`
	SCY  string  `json:"scy"`
	Net  string  `json:"net"`
	Host string  `json:"host"`
	Path string  `json:"path"`
	TLS  string  `json:"tls"`
	SNI  string  `json:"sni"`
	ALPN string  `json:"alpn"`
	FP   string  `json:"fp"`
}

// vmess://<base64 json>
func ParseVMess(uri string) (proxy.Node, error) {
	payload := strings.TrimPrefix(uri, "vmess://")
	payload, _, _ = strings.Cut(payload, "#")

	data, err := Unbase64(payload)
	if err != nil {
		return proxy.Node{}, fmt.Errorf("vmess: base64: %w", err)
	}
	var vm vmessLink
	if err := json.Unmarshal(data, &vm); err != nil {
		return proxy.Node{}, fmt.Errorf("vmess: json: %w", err)
	}
	if vm.Add == "" || vm.Port == 0 || vm.ID == "" {
		return proxy.Node{}, fmt.Errorf("vmess: missing add/port/id")
	}

	net := strings.ToLower(cmp.Or(vm.Net, "tcp"))
	n := proxy.Node{
		ID:       id(uri),
		Name:     cmp.Or(vm.PS, vm.Add),
		Protocol: proxy.VMess,
		Server:   vm.Add,
		Port:     int(vm.Port),
		Auth: proxy.Auth{
			UUID:    vm.ID,
			AlterID: int(vm.AID),
			Method:  strings.ToLower(cmp.Or(vm.SCY, "auto")),
		},
		Transport: proxy.Transport{
			Network: net,
			Path:    vm.Path,
			Host:    cmp.Or(vm.Host, vm.SNI),
		},
	}
	if net == "grpc" {
		n.Transport.ServiceName = vm.Path // grpc exports reuse "path" as name
	}
	if tls := strings.ToLower(vm.TLS); tls == "tls" || tls == "reality" {
		n.TLS = &proxy.TLS{
			SNI:         cmp.Or(vm.SNI, vm.Host, vm.Add),
			ALPN:        splitComma(vm.ALPN),
			Fingerprint: vm.FP,
		}
	}
	return n, nil
}
