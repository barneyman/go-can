package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"go-can/canbus_archiver"
	"go-can/canbus_native"
	"io"
	"log"
	"os/exec"
	"time"

	"github.com/go-daq/canbus"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stianeikeland/go-rpio"
)

func init() {
	gob.Register(canbus.Frame{})
	gob.Register(canbus_archiver.TimeStampedFrame{})

}

type canbusHandler interface {
	Bind(string) error
	Close()
	Recv() (canbus.Frame, error)
	Send(canbus.Frame) error
}

type shutdownRequest struct {
	Mandatory bool // think - battery voltage too low, you MUST exit
	Seconds   int  // how many seconds until i shutdown
}

func handlePower(canbus_exit chan string) error {

	err := rpio.Open()

	if err != nil {
		return err
	}

	defer rpio.Close()

	ctx := context.Background()

	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		return err
	}

	defer rdb.Close()

	redissub := rdb.Subscribe(ctx, "shutdown_deny")

	defer redissub.Close()

	ignitionPin := rpio.Pin(12)
	latchPin := rpio.Pin(25)

	ignitionPin.Mode(rpio.Input)
	latchPin.Mode(rpio.Output)

	latchPin.High()

	ignChan := make(chan rpio.State)

	timeBetweenIGNchecks := time.Second

	go func() {

		previousIgnState := rpio.High

		for {
			ignState := ignitionPin.Read()

			if previousIgnState != ignState {
				log.Printf("ign switched from %v to %v", previousIgnState, ignState)
				previousIgnState = ignState
				// send it on
				ignChan <- ignState
			}

			time.Sleep(timeBetweenIGNchecks)

		}

	}()

	timeBeforeShutdown := 30
	timeBeforeQuery := timeBeforeShutdown / 2
	for {

		// create stopped, benign timers
		queryTimer := time.NewTimer(time.Minute)
		queryTimer.Stop()

		shutdownTimer := time.NewTimer(time.Minute)
		shutdownTimer.Stop()

		for {
			select {
			case ignState := <-ignChan:
				if ignState == rpio.Low {
					// ign off, ask everybody if it's ok to shutdown in queryTimer time
					log.Print("Ign turned off, starting query timer")
					queryTimer.Reset(time.Second * time.Duration(timeBeforeQuery))
				} else {
					log.Print("Ign turned on, killing all timers")
					// we're back on, cancel all timers
					queryTimer.Stop()
					shutdownTimer.Stop()
				}

			case <-queryTimer.C:
				// craft the request
				sdr := shutdownRequest{Mandatory: false, Seconds: timeBeforeShutdown}
				jsonData, _ := json.Marshal(sdr)
				log.Print("Requesting permission to shutdown ...")
				err = rdb.Publish(ctx, "shutdown", string(jsonData)).Err()
				if err != nil {
					log.Print(err)
				}

				// restart the timer - if i don't hear from anyone ....
				log.Print("waiting for any responses ...")
				shutdownTimer.Reset(time.Second * time.Duration(timeBeforeShutdown))

			case <-shutdownTimer.C:

				log.Print("Shutdown timer expired, stopping canbus")

				// tell can to bail gracefully
				canbus_exit <- "canbus-end"
				time.Sleep(time.Second)

				log.Print("proceeding to system shutdown request")
				// proceed to shutdown
				//shutdown()
				return nil

			case <-redissub.Channel():
				// redis has sent a message
				log.Print("shutdown deferred")

				shutdownTimer.Stop()
				queryTimer.Reset(time.Second * time.Duration(timeBeforeQuery))
			}

		}

	}
}

func main() {

	canbus_exit := make(chan string)

	//canbussrc, err := canbus_test.New()
	//canbussrc, err := canbus_archive.New()
	canbussrc, err := canbus_native.New()
	if err != nil {
		log.Print(err.Error())
	}

	//canbusdst, err := canbus_null.New()
	canbusdst, err := canbus_archiver.New()
	if err != nil {
		log.Print(err.Error())
	}
	// defer canbussrc.Close()
	// defer canbusdst.Close()

	// & because Recv receives a pointer
	go can_dump(&canbussrc, &canbusdst, canbus_exit)

	// look after the power
	err = handlePower(canbus_exit)

	log.Print("main exiting")

	if err != nil {
		log.Fatal(err.Error())

	}

}

func shutdown() {

	cmd := exec.Command("sudo", "shutdown", "-h", "now")

	// Run the command and capture any errors.
	err := cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute shutdown command: %v", err)
	}

}

func serialiseFrame(frame canbus.Frame) ([]byte, error) {

	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode((frame))

	return b.Bytes(), err

}

// func deserialiseFrame(sframe []byte) (canbus.Frame, error) {

// 	m := canbus.Frame{}
// 	d := gob.NewDecoder(bytes.NewReader(sframe))

// 	err := d.Decode(&m)

// 	return m, err

// }

func can_dump(canbusSrc, canbusDst canbusHandler, exit chan string) {

	// vital thse are closed sanely
	defer canbusSrc.Close()
	defer canbusDst.Close()

	err := canbusSrc.Bind("can0")
	//err := canbusSrc.Bind("/var/canbus")

	if err != nil {
		log.Fatal(err.Error())
	}

	newuuid := uuid.New()
	filename := "/var/canbus/" + newuuid.String() + ".can"

	err = canbusDst.Bind(filename)
	if err != nil {
		log.Fatal(err.Error())
	}

	readChan := make(chan canbus.Frame)

	go func() {

		for {
			msg, err := canbusSrc.Recv()
			if err != nil {
				log.Print(err.Error())
				if errors.Is(err, io.EOF) {
					return
				}

			} else {
				readChan <- msg
			}
		}
	}()

	log.Print("canbus main loop")

	for {
		select {
		case <-exit:
			log.Print("can_dump exiting")
		case msg := <-readChan:
			canbusDst.Send(msg)
			if err == nil {
				b, _ := serialiseFrame(msg)
				log.Print(b)
			}
		}
	}

}
