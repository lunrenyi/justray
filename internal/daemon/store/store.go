package store

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"time"

	"github.com/luynrs/justray/internal/shared/domain"
	"gopkg.in/yaml.v3"
)

type Subscription struct {
	ID        string         `yaml:"id"`
	Name      string         `yaml:"name"`
	URL       string         `yaml:"url"`
	Nodes     []domain.Node  `yaml:"nodes"`
	UpdatedAt time.Time      `yaml:"updated_at"`
	Traffic   domain.Traffic `yaml:"traffic,omitempty"`
}

type State struct {
	Active string `yaml:"active"`
	Tun    bool   `yaml:"tun,omitempty"`
}

// Disk is the persistence boundary: subscriptions.yaml + the daemon's
// active-node/tun state.
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

func (d Disk) State() (State, error) {
	data, err := os.ReadFile(statePath(d.Dir))
	if err != nil {
		return State{}, skipMissing(err)
	}
	var s State
	return s, yaml.Unmarshal(data, &s)
}

func (d Disk) Active() (string, error) {
	s, err := d.State()
	return s.Active, err
}

func (d Disk) SetActive(id string) error {
	s, _ := d.State()
	s.Active = id
	data, _ := yaml.Marshal(s)
	return write(statePath(d.Dir), data)
}

func (d Disk) Tun() (bool, error) {
	s, err := d.State()
	return s.Tun, err
}

func (d Disk) SetTun(on bool) error {
	s, _ := d.State()
	s.Tun = on
	data, _ := yaml.Marshal(s)
	return write(statePath(d.Dir), data)
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
	var b [4]byte
	rand.Read(b[:]) // documented never to fail
	return hex.EncodeToString(b[:])
}

func subsPath(dir string) string  { return filepath.Join(dir, "subscriptions.yaml") }
func statePath(dir string) string { return filepath.Join(dir, "configuration.yaml") }
