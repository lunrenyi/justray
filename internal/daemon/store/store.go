package store

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luynrs/justray/internal/parser/proxy"
	"gopkg.in/yaml.v3"
)

type Subscription struct {
	ID        string       `yaml:"id"`
	Name      string       `yaml:"name"`
	URL       string       `yaml:"url"`
	Nodes     []proxy.Node `yaml:"nodes"`
	UpdatedAt time.Time    `yaml:"updated_at"`
	Traffic   Traffic      `yaml:"traffic,omitempty"`
}

type Traffic struct {
	UploadBytes   int64     `yaml:"upload_bytes,omitempty"`
	DownloadBytes int64     `yaml:"download_bytes,omitempty"`
	TotalBytes    int64     `yaml:"total_bytes,omitempty"`
	ExpiresAt     time.Time `yaml:"expires_at,omitempty"`
}

type Disk struct{ Dir string }

type file struct {
	Subscriptions []Subscription `yaml:"subscriptions"`
}

func (d Disk) Subscriptions() ([]Subscription, error) {
	data, err := os.ReadFile(subsPath(d.Dir))
	if err != nil {
		return nil, skipMissing(err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Subscriptions, nil
}

func (d Disk) Save(subs []Subscription) error {
	data, err := yaml.Marshal(file{subs})
	if err != nil {
		return err
	}
	return write(subsPath(d.Dir), data)
}

// the node id to reconnect to, "" if none
func (d Disk) Active() (string, error) {
	data, err := os.ReadFile(activePath(d.Dir))
	if err != nil {
		return "", skipMissing(err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (d Disk) SetActive(nodeID string) error {
	return write(activePath(d.Dir), []byte(nodeID))
}

func (d Disk) Tun() (bool, error) {
	data, err := os.ReadFile(tunPath(d.Dir))
	if err != nil {
		return false, skipMissing(err)
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

func (d Disk) SetTun(enabled bool) error {
	v := "0"
	if enabled {
		v = "1"
	}
	return write(tunPath(d.Dir), []byte(v))
}

func skipMissing(err error) error {
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func write(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func NewID() string {
	var b [8]byte
	rand.Read(b[:]) // documented never to fail
	return hex.EncodeToString(b[:])
}

func subsPath(dir string) string   { return filepath.Join(dir, "subscriptions.yaml") }
func activePath(dir string) string { return filepath.Join(dir, "active") }
func tunPath(dir string) string    { return filepath.Join(dir, "tun") }
