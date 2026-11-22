package Power

import (
	"context"
	"errors"
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

func NewRPICarPiHatPower(graceShutdown, ignCheckTimout time.Duration) *RPICarPiHatPower {
	return &RPICarPiHatPower{
		ignCheckTimeout:    ignCheckTimout,
		graceShutdownTimer: graceShutdown,
	}
}

func (rm *RPICarPiHatPower) GetExitPowerChannel() chan bool {
	return rm.exitPower
}

func (rm *RPICarPiHatPower) Init() error {
	// initialize the power monitor
	err := rpio.Open()
	if err != nil {
		return err
	}
	rm.exitPower = make(chan bool)

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

	// read ignition, if it's off we should fail, this is probably a bench test
	if rm.ignitionPin.Read() != rpio.High {
		return errors.New("ignition is off, expected it to be on")
	}

	return nil
}

func (rm *RPICarPiHatPower) GetExitPower() chan bool {
	return rm.exitPower
}

// this will be launched as a goroutine to monitor the power state and handle shutdown requests
func (rm *RPICarPiHatPower) Run(ctx context.Context, wg *sync.WaitGroup) {

	// we have to assume ignition was high recently, or we wouldn't have power, so we can start with that assumption
	lastIgnitionState := true

	ignStateChan := make(chan bool)

	wg.Go(func() {

		for running := true; running; {
			select {
			case <-ctx.Done():
				running = false
			default:
				// read the ignition pin
				log.Print("Checking ignition state of pin ")
				ignitionState := rm.ignitionPin.Read() == rpio.High

				if ignitionState != lastIgnitionState {
					log.Printf("Ignition state changed to: %v", ignitionState)
					// ignition state changed
					// send it down the channel
					ignStateChan <- ignitionState
					// remember the last state
					lastIgnitionState = ignitionState
				}
				time.Sleep(rm.ignCheckTimeout)
			}

		}
		log.Print("Ignition watch exiting")
	})

	withinShutdownGracePeriod := false
	shutdownGraceTimer := time.NewTimer(time.Minute)
	shutdownGraceTimer.Stop()

	// monitor ignition state and handle shutdown grace periods
	for running := true; running; {

		select {
		case <-ctx.Done():
			// TODO: we either exit cleanly or shutdown the underlying machine
			running = false
		case ignStateChangedTo := <-ignStateChan:
			// what was seen
			if ignStateChangedTo == false {
				// we've gone to off, start the grace period
				if withinShutdownGracePeriod {
					log.Fatal("shouldn't see it go to off while in grace period")
				}
				shutdownGraceTimer = time.NewTimer(rm.graceShutdownTimer)
				log.Printf("Ignition turned off, starting shutdown grace period")
			} else {
				log.Printf("Ignition turned back on, stopping shutdown grace period")
				// stop any grace timer we have
				withinShutdownGracePeriod = false
				shutdownGraceTimer.Stop()
			}
		case <-shutdownGraceTimer.C:
			// grace has expired ... tell dad we WANT to turn the power off
			// we will wait for the ctx before we actually exit, gives other processes
			// to exit cleanly
			log.Print("Shutdown grace period expired, requesting power off")
			rm.GetExitPower() <- true
			running = false
		}
	}
	log.Print("RPICarPiHatPower.Run exiting")

}

func (rm *RPICarPiHatPower) Close() error {
	return rpio.Close()
}
