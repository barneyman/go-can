package canbus

import (
	"context"
	"errors"
	"go-can/internal/canbus/canbus_archive"
	"go-can/internal/canbus/canbus_archiver"
	"go-can/internal/canbus/canbus_native"
	"go-can/internal/canbus/canbus_null"
	"go-can/internal/canbus/canbus_test"
	"sync"
	"time"

	"log"

	"github.com/go-daq/canbus"
)

const (
	CbmSrcNamedSource = iota
	CbmSrcarchive
	CbmSrcTest
	CbmSrcVirtual
)

const (
	CbmDstarchive = iota
	CbmDstNull
)

type CanbusHandler interface {
	BindDefault() error
	SetListenOnly(bool)
	Bind(string) error
	Close()
	Recv() (canbus.Frame, error)
	Send(canbus.Frame) error
}

type CanbusMonitor struct {
	canbussrc, canbusdst CanbusHandler
	C                    chan canbus.Frame
}

func NewCanbusMonitor() *CanbusMonitor {
	return &CanbusMonitor{
		C: make(chan canbus.Frame, 10),
	}
}

func (cbm *CanbusMonitor) Init(srcType, dstType int, srcMeta, dstMeta any) error {

	var err error

	switch srcType {
	case CbmSrcNamedSource:
		cbm.canbussrc, err = canbus_native.New()
	case CbmSrcarchive:
		cbm.canbussrc, err = canbus_archive.New()
	case CbmSrcTest:
		cbm.canbussrc, err = canbus_test.New()
	case CbmSrcVirtual:
		cbm.canbussrc, err = canbus_native.NewVirtual()
	default:
		err = errors.New("unknown CanBus src type")
	}

	if err != nil {
		return err
	}

	cbm.canbussrc.BindDefault()

	switch dstType {
	case CbmDstarchive:
		cbm.canbusdst, err = canbus_archiver.New()
	case CbmDstNull:
		cbm.canbusdst, err = canbus_null.New()
	default:
		err = errors.New("unknown CanBus dst type")
	}

	if err != nil {
		return err
	}

	cbm.canbusdst.BindDefault()

	return err
}

// launched as a go routine
func (cbm *CanbusMonitor) Run(ctx context.Context, wg *sync.WaitGroup) {

	// add my interest
	wg.Add(1)
	defer wg.Done()

	forcedClose := false

	go func() {
		<-ctx.Done()
		// barbaric but effective way to force the loop to exit
		forcedClose = true
		cbm.canbussrc.Close()
		cbm.canbusdst.Close()
	}()

	for running := true; running; {
		msg, err := cbm.canbussrc.Recv()
		if err != nil {
			if forcedClose {
				log.Print("Sockercan closed underneath me to force an exit")
				running = false
			} else {
				log.Print(err.Error())
			}
		} else {
			select {
			case cbm.C <- msg:
			case <-time.After(200 * time.Millisecond):
				log.Print("Timed-out sending frame through channel")
			}
			cbm.canbusdst.Send(msg)
		}

		//}
	}

	log.Print("CanbusMonitor.Run exiting")

}

func (cbm *CanbusMonitor) Send(frame canbus.Frame) error {
	return cbm.canbussrc.Send(frame)
}

func (cbm *CanbusMonitor) Close() error {
	// cbm.canbussrc.Close()
	// cbm.canbusdst.Close()
	return nil
}
