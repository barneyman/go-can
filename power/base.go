package Power

import (
	"context"
	"sync"
)

type RunMonitor interface {
	// fields for the power monitor
	GetExitPowerChannel() chan bool
	Init() error
	Run(ctx context.Context, wg *sync.WaitGroup)
	Close() error
}
