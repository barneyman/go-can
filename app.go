package main

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"errors"
	"go-can/canbus_archive"
	"go-can/canbus_archiver"
	"go-can/canbus_native"
	"go-can/canbus_null"
	"go-can/canbus_test"
	"go-can/odb_handler"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/barneyman/go-rpio/v4"
	"github.com/go-daq/canbus"
	"github.com/redis/go-redis/v9"
)

func init() {
	gob.Register(canbus.Frame{})
	gob.Register(canbus_archiver.TimeStampedFrame{})

}

type shutdownRequest struct {
	Mandatory bool // think - battery voltage too low, you MUST exit
	Seconds   int  // how many seconds until i shutdown
}

func handlePower(canbus_exit_channel chan string) error {

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

	for retry := 0; retry < 15; retry++ {
		_, err = rdb.Ping(ctx).Result()
		if err != nil {
			time.Sleep(time.Second * 2)
			log.Print("Trying to ping redis again")
		} else {
			break
		}

	}

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

	ignStateChannel := make(chan rpio.State)

	timeBetweenIGNchecks := time.Millisecond * 250

	go func() {

		previousIgnState := rpio.High

		for {
			ignState := ignitionPin.Read()

			if previousIgnState != ignState {
				log.Printf("ign switched from %v to %v", previousIgnState, ignState)
				previousIgnState = ignState
				// send it on
				ignStateChannel <- ignState
			}

			time.Sleep(timeBetweenIGNchecks)

		}

	}()

	timeBeforeShutdown := 5
	timeBeforeQuery := timeBeforeShutdown

	// allow us to fake timeouts during test data creation
	if os.Getenv("SHUTDOWN_IN_SECONDS") != "" {
		timeBeforeQuery, err = strconv.Atoi(os.Getenv("SHUTDOWN_IN_SECONDS"))
	}

	// sanity check
	if timeBeforeQuery == 0 || err != nil {
		timeBeforeQuery = timeBeforeShutdown
	}

	for {

		// create stopped, benign timers
		queryTimer := time.NewTimer(time.Minute)
		queryTimer.Stop()

		shutdownTimer := time.NewTimer(time.Minute)
		shutdownTimer.Stop()

		for {
			select {
			case ignState := <-ignStateChannel:
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
				canbus_exit_channel <- "canbus-end"
				time.Sleep(time.Second)

				log.Print("proceeding to system shutdown request")
				// proceed to shutdown
				return nil

			case <-redissub.Channel():
				// redis has sent a message
				log.Print("shutdown deferred")
				shutdownTimer.Stop()
				log.Print("shutdown timer reset")
				queryTimer.Reset(time.Second * time.Duration(timeBeforeQuery))
			}

		}

	}
}

func main() {

	canbus_exit_channel := make(chan string)

	var canbussrc, canbusdst odb_handler.CanbusHandler
	var err error

	// check what we're being asked for
	namedSrc := os.Getenv("CANSRC")
	namedDst := os.Getenv("CANDEST")
	listenOnly := false
	if os.Getenv("LISTEN_ONLY") != "" {
		listenOnly = true
	}

	if listenOnly {
		log.Print("LISTEN_ONLY mode enabled")
	}

	switch namedSrc {
	case "native":
		canbussrc, err = canbus_native.New()
	case "archive":
		canbussrc, err = canbus_archive.New()
	case "test":
		canbussrc, err = canbus_test.New()
	case "vcan":
		canbussrc, err = canbus_native.NewVirtual()
	default:
		log.Fatal("no CANSRC requested")
	}

	if err != nil {
		log.Print(err.Error())
	}

	canbussrc.SetListenOnly(listenOnly)

	switch namedDst {
	case "archive":
		canbusdst, err = canbus_archiver.New()
	case "null":
		canbusdst, err = canbus_null.New()
	default:
		log.Fatal("no CANDEST requseted")
	}

	if err != nil {
		log.Print(err.Error())
	}

	// & because Recv receives a pointer
	go can_dump(canbussrc, canbusdst, !listenOnly, canbus_exit_channel)

	for {

	}
	// look after the power
	err = handlePower(canbus_exit_channel)

	log.Print("main exiting")

	if err != nil {
		log.Print("handlePower error " + err.Error())

	}

	// otherwise shutdown
	if os.Getenv("SHUTDOWN") != "" {
		shutdown()
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

func can_dump(canbusSrc, canbusDst odb_handler.CanbusHandler, sendPids bool, exit chan string) {

	// vital thse are closed sanely
	defer canbusSrc.Close()
	defer canbusDst.Close()

	err := canbusSrc.BindDefault()

	if err != nil {
		log.Fatal(err.Error())
	}

	err = canbusDst.BindDefault()
	if err != nil {
		log.Fatal(err.Error())
	}

	myPIDhandler := odb_handler.PIDHandler{CbHandler: canbusSrc, SendPids: sendPids}

	readChan := make(chan canbus.Frame)
	/*
		// 2 len data bytes 1 mode 0xd vehicle rpm 5 bytes padding to 8
		reqData := []byte{2, 1, 0xc, 0, 0, 0, 0, 0}

		// 2 len data bytes 1 mode 0x2f fuel level 5 bytes padding to 8
		// reqData = []byte{2, 1, 0xd, 0, 0, 0, 0, 0}

		// response is
		// [src ID 7e8] [len] [mode | 0x40] [pid] [result]

		// RPM
		request := canbus.Frame{ID: 0x7df, Data: reqData, Kind: canbus.SFF}

		canbusSrc.Send(request)
	*/

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

	log.Print("canbus main loop starting ...")

	// important info
	CriticalInfoRequestTicker := time.NewTicker(24 * time.Hour)
	defer CriticalInfoRequestTicker.Stop()

	// less important
	LessCriticalInfoRequestTicker := time.NewTicker(24 * time.Hour)
	defer LessCriticalInfoRequestTicker.Stop()

	pidSupportedTicker := time.NewTicker(2 * time.Second)
	defer pidSupportedTicker.Stop()

outerLoop:
	for {
		select {
		case <-exit:
			log.Print("can_dump exiting")
			break outerLoop
		case msg := <-readChan:

			_, err := myPIDhandler.Parse(msg)

			if err != nil {
				log.Print(err.Error())
				continue
			}

			// send it to the archiver
			if canbusDst.Send(msg) == nil {
				frameBytes, _ := serialiseFrame(msg)
				log.Print(string(frameBytes))
			}

		case <-LessCriticalInfoRequestTicker.C:

			myPIDhandler.GetFuelLevel()
			myPIDhandler.GetEngineRuntimeS()

		case <-CriticalInfoRequestTicker.C:
			// send out requests
			myPIDhandler.GetRPM()
			myPIDhandler.GetSpeedKPH()
			myPIDhandler.GetThrottlePos()

		case <-pidSupportedTicker.C:

			switch myPIDhandler.ScanStarted {

			case odb_handler.NotStarted:
				log.Printf("Starting PID Support Query")
				// start the PID check
				err = myPIDhandler.GetSupportedPID(0)

				if err != nil {
					log.Print(err.Error())
				}
			case odb_handler.Complete:
				if myPIDhandler.SendPids {

					log.Printf("Starting Data Query ticker")
					CriticalInfoRequestTicker.Stop()
					CriticalInfoRequestTicker = time.NewTicker(100 * time.Millisecond)

					LessCriticalInfoRequestTicker.Stop()
					LessCriticalInfoRequestTicker = time.NewTicker(1 * time.Minute)
				}

				myPIDhandler.ScanStarted = odb_handler.Closed

			case odb_handler.Ongoing:
				log.Print("PID Support Query still ongoing")

			}

		}
	}

}
