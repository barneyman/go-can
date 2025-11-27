package main

import (
	"bytes"
	"encoding/gob"
	cambusnative "go-can/canbusnative"
	"log"
	"os"
	"time"

	"github.com/go-daq/canbus"
	"github.com/stianeikeland/go-rpio"
)

func init() {
	gob.Register(canbus.Frame{})
}

func main() {

	canbus_exit := make(chan string)

	canbus, err := cambusnative.New()

	if err != nil {
		log.Print(err.Error())
	}

	defer canbus.Close()

	go can_dump(canbus, canbus_exit)

	err = rpio.Open()

	if err != nil {
		log.Fatal(err.Error())
	}

	defer rpio.Close()

	ignitionPin := rpio.Pin(12)
	latchPin := rpio.Pin(25)
	lowTime := time.Second * 5
	var ignitionCounter time.Duration = 0

	ignitionPin.Mode(rpio.Input)
	latchPin.Mode(rpio.Output)

	latchPin.High()

	for {
		if ignitionPin.Read() != rpio.High {
			time.Sleep(time.Second)
			ignitionCounter++

			if time.Second*ignitionCounter > lowTime {

				log.Print("shutdown!")

				canbus_exit <- "exit"

				time.Sleep(time.Second)

				log.Print("shutdown!")
				//shutdown()
				return
			}

			log.Print("sleeping ...")

		} else {
			ignitionCounter = 0

		}
	}
}

// func shutdown() {

// 	cmd := exec.Command("sudo", "shutdown", "-h", "now")

// 	// Run the command and capture any errors.
// 	err := cmd.Run()
// 	if err != nil {
// 		log.Fatalf("Failed to execute shutdown command: %v", err)
// 	}

// }

func serialiseFrame(frame canbus.Frame) ([]byte, error) {

	b := bytes.Buffer{}
	e := gob.NewEncoder(&b)
	err := e.Encode((frame))

	return b.Bytes(), err

}

func deserialiseFrame(sframe []byte) (canbus.Frame, error) {

	m := canbus.Frame{}
	d := gob.NewDecoder(bytes.NewReader(sframe))

	err := d.Decode(&m)

	return m, err

}

func can_dump(canbusSrc cambusnative.CanbusnativeData, exit chan string) {

	// test
	frame := canbus.Frame{ID: 1, Data: []byte("HACK"), Kind: canbus.EFF}

	sFrame, _ := serialiseFrame(frame)

	rFrame, _ := deserialiseFrame(sFrame)

	log.Print(rFrame)

	log.Print(string(sFrame))

	err := canbusSrc.Bind("can0")

	if err != nil {
		log.Fatal(err.Error())
	}

	cb, err := os.Create("/tmp/wibble.canbus")

	if err != nil {
		log.Fatal(err.Error())
	}

	defer cb.Close()

	readChan := make(chan canbus.Frame)

	go func() {

		for {
			msg, err := canbusSrc.Recv()
			if err != nil {
				log.Print(err.Error())

			} else {
				readChan <- msg
			}
		}
	}()

	log.Print("looping")

	for {
		select {
		case <-exit:
			log.Print("can_dump ending")
		case read := <-readChan:
			b, err := serialiseFrame(read)
			if err == nil {
				log.Print(b)
				cb.Write(b)
			}
		}
	}
}
