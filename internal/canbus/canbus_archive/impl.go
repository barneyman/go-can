package canbus_archive

import (
	"encoding/gob"
	"errors"
	"io"
	"log"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-daq/canbus"

	"go-can/internal/canbus/canbus_archiver"
)

// archive reader
type Canbusnativearchive_Data struct {
	handle       *os.File
	decoder      *gob.Decoder
	archives     []os.DirEntry
	archiveIndex int
}

func New() (*Canbusnativearchive_Data, error) {

	return &Canbusnativearchive_Data{archiveIndex: 0}, nil

}

func (cs *Canbusnativearchive_Data) SetListenOnly(listenOnly bool) {
	// do nothing, this is a read-only archive
}

func (cs *Canbusnativearchive_Data) BindDefault() error {

	// get list of files from data and get the newest one

	sourceDir := "./data"

	files, err := os.ReadDir(sourceDir)

	if err != nil {
		return err
	}

	// first prune out non-files
	files = slices.DeleteFunc(files, func(e os.DirEntry) bool {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			return true
		}
		info, err := e.Info()
		if err != nil {
			return true
		}
		if info.Size() == 0 {
			return true
		}

		return false
	})

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
		return infoI.ModTime().Before(infoJ.ModTime())

		// To sort in descending order (newest first), use:
		// return infoJ.ModTime().Before(infoI.ModTime())

	})

	cs.archives = files

	err = cs.Bind(sourceDir + "/" + files[cs.archiveIndex].Name())
	if err != nil {
		return err
	}
	cs.archiveIndex++
	return nil
}

func (cs *Canbusnativearchive_Data) Bind(name string) error {

	handle, err := os.Open(name)
	if err != nil {
		return err
	}
	log.Printf("Archive binding to %s", name)
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

	if errors.Is(err, io.EOF) {
		// next file if there is one
		if cs.archiveIndex < len(cs.archives) {
			cs.Close()
			err = cs.Bind("./data/" + cs.archives[cs.archiveIndex].Name())
			if err != nil {
				return canbus.Frame{}, err
			}
			cs.archiveIndex++
			return cs.Recv()
		}
		return canbus.Frame{}, io.EOF
	}

	time.Sleep(tsf.Offset)

	return tsf.Frame, err

}

func (cs *Canbusnativearchive_Data) Send(msg canbus.Frame) error {

	log.Print("Send not implemented in archive")
	return nil

}
