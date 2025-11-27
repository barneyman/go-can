package cambus_archive

import (
	"encoding/gob"
	"os"

	"github.com/go-daq/canbus"
)

type Canbusnativearchive_Data struct {
	handle  *os.File
	decoder *gob.Decoder
}

func New() (Canbusnativearchive_Data, error) {

	return Canbusnativearchive_Data{}, nil

}

func (cs *Canbusnativearchive_Data) Bind(name string) error {

	handle, err := os.Open(name)
	cs.handle = handle

	cs.decoder = gob.NewDecoder(cs.handle)

	return err
}

func (cs *Canbusnativearchive_Data) Close() {
	cs.handle.Close()
}

func (cs *Canbusnativearchive_Data) Recv() (canbus.Frame, error) {

	msg := canbus.Frame{}

	err := cs.decoder.Decode(&msg)

	return msg, err

}
