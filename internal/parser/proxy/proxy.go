package proxy

type Proto string

const (
	VMess  Proto = "vmess"
	VLess  Proto = "vless"
	Trojan Proto = "trojan"
	SS     Proto = "shadowsocks"
	HY2    Proto = "hysteria2"
)

type Node struct {
	ID        string
	Name      string
	Protocol  Proto
	Server    string
	Port      int
	Auth      Auth
	Transport Transport
	TLS       *TLS // nil means plaintext
	Reality   *Reality
	Obfs      string // hysteria2 salamander, can't proceed via xray :9
}

type Auth struct {
	UUID     string // vmess, vless
	Password string // trojan, ss, hysteria2
	Method   string // ss cipher, vmess security
	Flow     string // vless, e.g. xtls
	AlterID  int    // legacy vmess
}

type Transport struct {
	Network     string // tcp, ws, grpc, quic
	Path        string
	Host        string
	ServiceName string // grpc
}

type TLS struct {
	SNI         string
	ALPN        []string
	Fingerprint string
	Insecure    bool
}

type Reality struct {
	PublicKey string
	ShortID   string
	SpiderX   string
}
