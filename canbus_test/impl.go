package cambus_test

import (
	"errors"

	"github.com/go-daq/canbus"
)

type Canbusnativetest_Data struct {
	id uint32
}

func New() (Canbusnativetest_Data, error) {

	cs := Canbusnativetest_Data{id: 1}

	return cs, nil

}

func (cs Canbusnativetest_Data) Bind(name string) error {

	return nil
}

func (cs Canbusnativetest_Data) Close() {
}

func (cs *Canbusnativetest_Data) Recv() (canbus.Frame, error) {

	frame := canbus.Frame{ID: cs.id, Data: []byte("HACK"), Kind: canbus.EFF}
	cs.id = cs.id + 1

	return frame, errors.New("cannot recv")

}
