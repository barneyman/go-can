package canbus_native

import (
	"errors"

	"github.com/go-daq/canbus"
)

//////////////////////////////////////////////////////////////////////////////

type CanbusnativeData struct {
	sock       *canbus.Socket
	listenOnly bool
}

func New() (*CanbusnativeData, error) {
	sock, err := canbus.New()

	cs := CanbusnativeData{sock: sock, listenOnly: false}

	return &cs, err

}

func (cs *CanbusnativeData) BindDefault() error {

	return cs.Bind("can0")
}

func (cs *CanbusnativeData) SetListenOnly(listenOnly bool) {
	cs.listenOnly = listenOnly

}

func (cs *CanbusnativeData) Bind(name string) error {

	if cs.sock == nil {
		return errors.New("CanbusnativeData sock not valid")
	}

	return cs.sock.Bind(name)
}

func (cs CanbusnativeData) Close() {
	cs.sock.Close()
}

func (cs CanbusnativeData) Recv() (canbus.Frame, error) {

	if cs.sock != nil {
		return cs.sock.Recv()
	}

	return canbus.Frame{}, errors.New("cannot recv")

}

func (cs *CanbusnativeData) Send(msg canbus.Frame) error {

	if cs.listenOnly {
		return errors.New("cannot send in listen-only mode")
	}

	if cs.sock != nil {
		_, err := cs.sock.Send(msg)
		return err
	}

	return errors.New("sock nil")

}

// =====================================================================

type canbusnativeVirtual struct {
	CanbusnativeData
}

func (cs *canbusnativeVirtual) BindDefault() error {

	return cs.Bind("vcan0")
}

func NewVirtual() (*canbusnativeVirtual, error) {

	cs, err := New()

	if err != nil {
		return nil, err
	}

	csv := canbusnativeVirtual{CanbusnativeData{sock: cs.sock}}

	return &csv, err

}
