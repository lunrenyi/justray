package engine

// New builds a fresh, unstarted Engine bound to a local proxy port and log
// path. engine/singbox.New is the only implementation; connection depends
// on this type instead of the concrete package.
type New func(port int, logPath string) Engine
