package Power

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/barneyman/go-rpio/v4"
)

type RPICarPiHatPower struct {

	// our specific things
	ignitionPin        rpio.Pin
	latchPin           rpio.Pin
	ignCheckTimeout    time.Duration
	graceShutdownTimer time.Duration
	exitPower          chan bool
}

func NewRPICarPiHatPower() *RPICarPiHatPower {
	return &RPICarPiHatPower{}
}

func (rm *RPICarPiHatPower) GetExitPowerChannel() chan bool {
	return rm.exitPower
}

func (rm *RPICarPiHatPower) Init(graceShutdown, ignCheckTimout time.Duration) error {
	// initialize the power monitor
	err := rpio.Open()
	if err != nil {
		return err
	}
	rm.exitPower = make(chan bool)

	rm.ignCheckTimeout = ignCheckTimout
	rm.graceShutdownTimer = graceShutdown

	// the pins we use
	rm.ignitionPin = rpio.Pin(12)
	rm.latchPin = rpio.Pin(25)

	// set their direction
	// ignition pin is input, latch pin is output
	// ignition is true if the car is running
	rm.ignitionPin.Input()
	// latch is for us to say - ok hat, keep the power on
	rm.latchPin.Output()

	// set the latch pin high to keep the power on
	rm.latchPin.High()

	return nil
}

func (rm *RPICarPiHatPower) GetExitPower() chan bool {
	return rm.exitPower
}

// this will be launched as a goroutine to monitor the power state and handle shutdown requests
func (rm *RPICarPiHatPower) Run(ctx context.Context, wg *sync.WaitGroup) {

	// add my interest
	wg.Add(1)
	defer wg.Done()

	// we have to assume ignition was high recently, or we wouldn't have power, so we can start with that assumption
	lastIgnitionState := true

	ignStateChan := make(chan bool)

	go func(context.Context) {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// read the ignition pin
				ignitionState := rm.ignitionPin.Read() == rpio.High

				if ignitionState != lastIgnitionState {
					// ignition state changed
					// send it down the channel
					ignStateChan <- ignitionState
					// remember the last state
					lastIgnitionState = ignitionState
				}
				time.Sleep(rm.ignCheckTimeout)
			}

		}
	}(ctx)

	withinShutdownGracePeriod := false
	shutdownGraceTimer := time.NewTimer(time.Minute)
	shutdownGraceTimer.Stop()

	// monitor ignition state and handle shutdown grace periods
	for {

		select {
		case <-ctx.Done():
			// TODO: we either exit cleanly or shutdown the underlying machine
			return
		case ignStateChangedTo := <-ignStateChan:
			// what was seen
			if ignStateChangedTo == false {
				// we've gone to off, start the grace period
				if withinShutdownGracePeriod {
					log.Fatal("shouldn't see it go to off while in grace period")
				}
				shutdownGraceTimer = time.NewTimer(rm.graceShutdownTimer)
			} else {
				if !withinShutdownGracePeriod {
					log.Fatal("shouldn't see it go to on while not in grace period")
				}
				// stop any grace timer we have
				withinShutdownGracePeriod = false
				shutdownGraceTimer.Stop()
			}
		case <-shutdownGraceTimer.C:
			// grace has expired ... tell dad we WANT to turn the power off
			// we will wait for the ctx before we actually exit, gives other processes
			// to exit cleanly
			rm.GetExitPower() <- true
		}
	}
	log.Print("RPICarPiHatPower.Run exiting")

}

func (rm *RPICarPiHatPower) Close() error {
	return rpio.Close()
}
