package Power

import (
	"context"
	"log"
	"sync"
	"time"
)

type NormalPower struct {
	exitPower chan bool
}

func NewNormalPower() *NormalPower {
	return &NormalPower{}
}

func (rm NormalPower) GetExitPowerChannel() chan bool {
	return rm.exitPower
}

func (rm *NormalPower) Init() error {
	// any initialization logic for normal power can go here
	rm.exitPower = make(chan bool)
	return nil
}

func (rm NormalPower) Run(ctx context.Context, wg *sync.WaitGroup) {

	shutdownTimer := time.NewTimer(time.Second * 60)

	// monitor the power state and handle shutdown requests
	for running := true; running; {
		select {
		case <-ctx.Done():
			running = false
		case <-shutdownTimer.C:
			rm.exitPower <- true
			running = false

		}
	}
	log.Print("NormalPower.Run exiting")
}

func (rm NormalPower) Close() error {
	// any cleanup logic for normal power can go here
	return nil
}
