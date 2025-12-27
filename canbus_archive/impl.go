package canbus_archive

import (
	"encoding/gob"
	"errors"
	"os"
	"sort"
	"time"

	"github.com/go-daq/canbus"

	"go-can/canbus_archiver"
)

// archive reader
type Canbusnativearchive_Data struct {
	handle  *os.File
	decoder *gob.Decoder
}

func New() (*Canbusnativearchive_Data, error) {

	return &Canbusnativearchive_Data{}, nil

}

func (cs *Canbusnativearchive_Data) BindDefault() error {

	// get list of files from data and get the newest one

	sourceDir := "./data"

	files, err := os.ReadDir(sourceDir)

	if err != nil {
		return err
	}

	if len(files) == 0 {
		return errors.New("no archives to use")
	}

	// Sort the slice of fs.DirEntry by modification time
	sort.Slice(files, func(i, j int) bool {
		infoI, errI := files[i].Info()
		infoJ, errJ := files[j].Info()

		// Basic error handling for getting file info
		if errI != nil || errJ != nil {
			// Handle errors as appropriate for your application.
			// For simplicity in this example, we return false to
			// maintain a consistent order if info is unavailable.
			return false
		}

		// Sort in ascending order (oldest first)
		//return infoI.ModTime().Before(infoJ.ModTime())

		// To sort in descending order (newest first), use:
		return infoJ.ModTime().Before(infoI.ModTime())

	})

	return cs.Bind(sourceDir + "/" + files[0].Name())
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

	tsf := canbus_archiver.TimeStampedFrame{}

	err := cs.decoder.Decode(&tsf)

	time.Sleep(tsf.Offset)

	return tsf.Frame, err

}

func (cs *Canbusnativearchive_Data) Send(msg canbus.Frame) error {

	return errors.New("not implemented")

}
