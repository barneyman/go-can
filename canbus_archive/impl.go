package canbus_archive

import (
	"encoding/gob"
	"errors"
	"os"
	"time"

	"github.com/go-daq/canbus"

	"go-can/canbus_archiver"
)

type TimeStampedFrame = canbus_archiver.TimeStampedFrame

// archive reader
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

	tsf := TimeStampedFrame{}

	err := cs.decoder.Decode(&tsf)

	time.Sleep(tsf.Offset)

	return tsf.Frame, err

}

func (cs *Canbusnativearchive_Data) Send(msg canbus.Frame) error {

	return errors.New("not implemented")

}
