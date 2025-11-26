package main

import (
	"log"
	"time"

	"github.com/go-daq/canbus"
	"github.com/stianeikeland/go-rpio"
)

func main() {

	canbus_exit := make(chan string)

	sock, err := canbus.New()

	if err != nil {
		log.Print(err.Error())
	}

	defer sock.Close()

	go can_dump(sock, canbus_exit)

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

func can_dump(sock *canbus.Socket, exit chan string) {

	if sock != nil {

		err := sock.Bind("can0")
		if err != nil {
			log.Fatal(err.Error())
		}
	}

	readChan := make(chan canbus.Frame)

	go func() {

		for {
			if sock != nil {
				msg, err := sock.Recv()

				if err != nil {
					log.Print(err.Error())

				} else {
					readChan <- msg
				}
			}
		}
	}()

	log.Print("looping")

	for {
		select {
		case <-exit:
			log.Print("can_dump ending")
		// case <-readChan:
		// 	log.Print("readchan")
		default:
			log.Print(".")
		}
	}
}
