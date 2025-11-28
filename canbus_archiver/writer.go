package canbus_archiver

import (
	"encoding/gob"
	"errors"
	"os"
	"time"

	"github.com/go-daq/canbus"
)

type Canbusnativearchiver_Data struct {
	handle            *os.File
	encoder           *gob.Encoder
	startAt, lastSend time.Time
}

type TimeStampedFrame struct {
	Offset time.Duration
	Frame  canbus.Frame
}

func New() (Canbusnativearchiver_Data, error) {

	return Canbusnativearchiver_Data{}, nil

}

func (cs *Canbusnativearchiver_Data) Bind(name string) error {

	handle, err := os.Create(name)
	cs.handle = handle

	cs.encoder = gob.NewEncoder(cs.handle)

	// got to watch for time slew underneath me
	cs.startAt = time.Now()
	cs.lastSend = time.Now()

	return err
}

func (cs *Canbusnativearchiver_Data) Close() {
	cs.handle.Close()
}

func (cs *Canbusnativearchiver_Data) Recv() (canbus.Frame, error) {
	return canbus.Frame{}, errors.New("not implemented")
}

func (cs *Canbusnativearchiver_Data) Send(msg canbus.Frame) error {

	thisOffset := time.Since(cs.lastSend)

	tsf := TimeStampedFrame{Offset: thisOffset, Frame: msg}
	err := cs.encoder.Encode(&tsf)

	cs.lastSend = time.Now()

	return err

}
