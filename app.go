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

type canbusHandler interface {
	BindDefault() error
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
				canbus_exit <- "canbus-end"
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

	canbus_exit := make(chan string)

	var canbussrc, canbusdst canbusHandler
	var err error

	// check what we're being asked for
	namedSrc := os.Getenv("CANSRC")
	namedDst := os.Getenv("CANDEST")

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
	go can_dump(canbussrc, canbusdst, canbus_exit)

	for {

	}
	// look after the power
	err = handlePower(canbus_exit)

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

type PIDsupported int
type PIDscanning int

const (
	UnknownSupport PIDsupported = iota
	Supported
	Unsupported
)

const (
	NotStarted PIDscanning = iota
	Ongoing
	Done
)

type PIDHandler struct {
	cbHandler      canbusHandler
	suppportedPIDs [255]PIDsupported
	ScanStarted    PIDscanning
}

type PIDresponse struct {
	respID  int
	service int
	PID     int
	data    []byte
}

const PID_RESPONSE_FLAG byte = 0x40
const OBD2_ID uint = 0x7DF

func (ph *PIDHandler) Parse(resp canbus.Frame) error {

	// check packet integrity
	pidlen := int(resp.Data[0])

	if len(resp.Data) <= pidlen {
		return errors.New("wrong size payload")
	}

	// confirm it's a response
	if resp.Data[1]&0x40 == 0 {
		return errors.New("not a response payload")
	}

	response := PIDresponse{respID: int(resp.ID), service: int(resp.Data[1] &^ PID_RESPONSE_FLAG), PID: int(resp.Data[2]), data: resp.Data[3:]}

	log.Printf("Parsing ID: %x Service %d PID: %x\r", response.respID, response.service, response.PID)

	if response.service != 1 {
		return errors.New("unsupported service")
	}

	switch response.PID {
	// handle supported PIDs
	case 0, 32, 64, 96, 128, 160, 192:
		if response.PID == 0 {
			ph.ScanStarted = Ongoing
		}
		// populate the array
		offset := 1 + (response.PID)

		for _, v := range response.data {
			for bit := 7; bit >= 0; bit-- {
				if (v & (1 << bit)) != 0 {
					ph.suppportedPIDs[offset] = Supported
				} else {
					ph.suppportedPIDs[offset] = Unsupported
				}
				offset++
			}
		}

		if response.PID < 192 {
			ph.GetSupportedPID(uint(response.PID) + 1)
		} else {
			ph.ScanStarted = Done
		}
		return nil
	}

	return nil
}

func (ph PIDHandler) sendOBD_PID_Current(PID uint, reqData []byte) error {

	return ph.sendPID(OBD2_ID, 1, PID, reqData)
}

func (ph PIDHandler) sendOBD_PID(Service, PID uint, reqData []byte) error {

	return ph.sendPID(OBD2_ID, Service, PID, reqData)
}

func (ph PIDHandler) sendPID(ID, Service, PID uint, reqData []byte) error {

	// check
	if ph.suppportedPIDs[PID] == Unsupported {
		return errors.New("unsupported pid")
	}

	frameData := []byte{byte(Service), byte(PID)}
	frameData = append(frameData, reqData...)

	cf := canbus.Frame{ID: uint32(ID), Data: frameData, Kind: canbus.SFF}

	return ph.cbHandler.Send(cf)

}

func (ph PIDHandler) GetSupportedPID(startAt uint) error {

	log.Printf("GetSupportedPID %d\r", startAt)

	// check startAt
	if startAt > 7 {
		return errors.New("PID request too high")
	}

	return ph.sendOBD_PID_Current(0, []byte{})
}

func (ph PIDHandler) GetRPM() error {
	return ph.sendOBD_PID_Current(0xC, []byte{})
}

func can_dump(canbusSrc, canbusDst canbusHandler, exit chan string) {

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

	myPIDhandler := PIDHandler{cbHandler: canbusSrc}

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

	log.Print("canbus main loop")

	for {
		select {
		case <-exit:
			log.Print("can_dump exiting")
		case msg := <-readChan:

			myPIDhandler.Parse(msg)

			// send it to the archiver
			if canbusDst.Send(msg) == nil {
				frameBytes, _ := serialiseFrame(msg)
				log.Print(string(frameBytes))
			}

		default:
			if myPIDhandler.ScanStarted == NotStarted {
				// start the PID check
				err = myPIDhandler.GetSupportedPID(0)

				if err != nil {
					log.Print(err.Error())
				}

			}

			time.Sleep(20 * time.Second)

		}
	}

}
