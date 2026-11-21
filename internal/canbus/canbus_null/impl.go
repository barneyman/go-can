package canbus_null

import (
	"log"

	"github.com/go-daq/canbus"
)

//////////////////////////////////////////////////////////////////////////////

type CanbusnullData struct {
}

func New() (*CanbusnullData, error) {

	cs := CanbusnullData{}

	return &cs, nil

}

func (cs *CanbusnullData) SetListenOnly(listenOnly bool) {
}

func (cs CanbusnullData) BindDefault() error {

	return nil
}

func (cs CanbusnullData) Bind(name string) error {

	return nil
}

func (cs CanbusnullData) Close() {

}

func (cs CanbusnullData) Recv() (canbus.Frame, error) {

	return canbus.Frame{}, nil

}

func (cs *CanbusnullData) Send(msg canbus.Frame) error {
	log.Print("Send not implemented in null")
	return nil

}
