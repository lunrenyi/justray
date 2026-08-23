package engine

import "github.com/luynrs/justray/internal/shared/domain"

type New func(s domain.Settings, logPath string) Engine

type Probe func(nodes []domain.Node, s domain.Settings, logPath string) (map[string]Result, error)
