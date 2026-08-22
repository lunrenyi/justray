package singbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	sbox "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/service"

	"github.com/luynrs/justray/internal/daemon/engine"
	"github.com/luynrs/justray/internal/daemon/engine/singbox/resolvers"
	"github.com/luynrs/justray/internal/daemon/platform/link"
	"github.com/luynrs/justray/internal/daemon/platform/wintun"
	"github.com/luynrs/justray/internal/shared/domain"
)

// Engine drives one running proxy/TUN session for a single node: it's the
// only concrete type in the codebase that reaches into sing-box directly.
type Engine struct {
	port    int
	logPath string

	inst  *sbox.Box
	iface string // non-empty while a TUN inbound is up
}

// New builds a fresh, unstarted Engine bound to a local proxy port and log path.
func New(port int, logPath string) engine.Engine {
	return &Engine{port: port, logPath: logPath}
}

func (e *Engine) Start(n domain.Node, iface string) error {
	opts, err := Build(n, e.port, e.logPath, iface)
	if err != nil {
		return err
	}

	inst, err := newBox(*opts)
	for i := 0; i < 2 && err != nil && iface != "" && errors.Is(err, syscall.EBUSY); i++ {
		if inst != nil {
			inst.Close()
		}
		waitGone(iface)
		inst, err = newBox(*opts)
	}
	if err != nil {
		if inst != nil {
			inst.Close()
		}
		return err
	}

	e.inst, e.iface = inst, iface
	return nil
}

func newBox(opts option.Options) (*sbox.Box, error) {
	inst, err := sbox.New(sbox.Options{Options: opts, Context: Context(context.Background())})
	if err == nil {
		err = inst.Start()
	}
	return inst, err
}

func (e *Engine) Swap(n domain.Node) error {
	ep, obs, err := Proxy(n)
	if err != nil {
		return err
	}

	ctx := e.runtimeCtx()
	router := e.inst.Router()
	logger := e.inst.LogFactory().NewLogger("outbound/" + Tag)

	if ep != nil {
		e.inst.Outbound().Remove(Tag)
		e.inst.Outbound().Remove(Tag + "-stls")
		return e.inst.Endpoint().Create(ctx, router, logger, ep.Tag, ep.Type, ep.Options)
	}
	e.inst.Endpoint().Remove(Tag)
	e.inst.Outbound().Remove(Tag + "-stls")
	for _, ob := range obs {
		if err := e.inst.Outbound().Create(ctx, router, logger, ob.Tag, ob.Type, ob.Options); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) TunAdd(iface string) error {
	if _, err := wintun.Ensure(); err != nil {
		return err
	}
	inb := TunInbound(iface, resolvers.Get())
	ctx := e.runtimeCtx()
	logger := e.inst.LogFactory().NewLogger("inbound/tun[tun-in]")

	var err error
	for i := 0; i < 3; i++ {
		err = e.inst.Inbound().Create(ctx, e.inst.Router(), logger, "tun-in", C.TypeTun, inb.Options)
		if err == nil || !errors.Is(err, syscall.EBUSY) {
			break
		}
		link.Delete(iface)
		waitGone(iface)
	}
	if err == nil {
		e.iface = iface
	}
	return err
}

func (e *Engine) TunRemove(iface string) error {
	err := e.inst.Inbound().Remove("tun-in")
	if waitGone(iface) {
		e.iface = ""
		return err
	}
	link.Delete(iface)
	if !waitGone(iface) {
		return fmt.Errorf("%s still up after removing tun-in", iface)
	}
	e.iface = ""
	return err
}

func (e *Engine) Close() error {
	if e.inst == nil {
		return nil
	}
	err := e.inst.Close()
	if e.iface != "" {
		waitGone(e.iface)
	}
	e.inst, e.iface = nil, ""
	return err
}

func (e *Engine) runtimeCtx() context.Context {
	return service.ContextWith[adapter.NetworkManager](Context(context.Background()), e.inst.Network())
}

func waitGone(iface string) bool {
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); {
		if _, err := net.InterfaceByName(iface); err != nil {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}
