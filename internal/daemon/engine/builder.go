package engine

import "github.com/luynrs/justray/internal/shared/domain"

type New func(port int, logPath string) Engine

type Probe func(nodes []domain.Node, logPath string) (map[string]Result, error)
