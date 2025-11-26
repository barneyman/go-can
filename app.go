package main

import (
	"log"
	"os/exec"
	"time"

	"github.com/go-daq/canbus"
	"github.com/stianeikeland/go-rpio"
)

func main() {
	// sock, err := canbus.New()

	// if err != nil {
	// 	log.Fatal(err.Error())
	// }

	// go can_dump(sock)

	err := rpio.Open()

	if err != nil {
		log.Fatal(err.Error())
	}

	defer rpio.Close()

	ignitionPin := rpio.Pin(12)
	latchPin := rpio.Pin(25)
	lowTime := time.Second * 10
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
				shutdown()
			}

			log.Print("sleeping ...")

		} else {
			ignitionCounter = 0

		}
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

func can_dump(sock *canbus.Socket) {

	log.Print("bail")
	return

	err := sock.Bind("can0")
	if err != nil {
		log.Fatal(err.Error())
	}

	for {
		msg, err := sock.Recv()

		if err != nil {
			log.Fatal(err.Error())
		}

		log.Print(msg)
	}

}
