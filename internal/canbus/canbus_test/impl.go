package canbus_test

import (
	"math/rand"
	"time"

	"github.com/go-daq/canbus"
)

type Canbusnativetest_Data struct {
	id uint32
}

func New() (*Canbusnativetest_Data, error) {

	cs := Canbusnativetest_Data{id: 1}

	return &cs, nil

}

func (cs Canbusnativetest_Data) BindDefault() error {

	return nil
}

func (cs *Canbusnativetest_Data) SetListenOnly(listenOnly bool) {
	// do nothing, this is a test archive
}

func (cs Canbusnativetest_Data) Bind(name string) error {

	return nil
}

func (cs Canbusnativetest_Data) Close() {
}

func (cs *Canbusnativetest_Data) Recv() (canbus.Frame, error) {

	// random buffer
	length := rand.Intn(32) + 12
	randomBytes := make([]byte, length)
	_, err := rand.Read(randomBytes)

	// wait a random amount
	millis := rand.Intn(5) + 2
	time.Sleep(time.Duration(millis) * time.Second)

	if err != nil {
		return canbus.Frame{}, err
	}

	frame := canbus.Frame{ID: cs.id, Data: randomBytes, Kind: canbus.EFF}
	cs.id = cs.id + 1

	return frame, nil

}

func (cs *Canbusnativetest_Data) Send(msg canbus.Frame) error {

	return nil

}
