package cambusnative

import (
	"errors"

	"github.com/go-daq/canbus"
)

type CanbusnativeData struct {
	sock *canbus.Socket
}

func New() (CanbusnativeData, error) {
	sock, err := canbus.New()

	cs := CanbusnativeData{sock: sock}

	return cs, err

}

func (cs CanbusnativeData) Bind(name string) error {

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
