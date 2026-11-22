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
	UnknownSrc = iota
	CbmSrcNamedSource
	CbmSrcarchive
	CbmSrcTest
	CbmSrcVirtual
)

const (
	UnknownDst = iota
	CbmDstarchive
	CbmDstNull
)

type CanbusHandler interface {
	BindDefault() error
	SetListenOnly(bool)
	Bind(string) error
	Close()
	SetRecvTimeout(time.Duration) error
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
	// set recv timeout
	cbm.canbussrc.SetRecvTimeout(200 * time.Millisecond)

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

	for running := true; running; {

		msg, err := cbm.canbussrc.Recv()
		if err != nil {
			if err != canbus.ErrReceiveTimeout {
				log.Print("Error receiving from canbussrc: ", err.Error())
				running = false
				// round again
				continue
			}
			// ErrReceiveTimeout has to fall through to see the Done case in the select below
			select {
			case <-ctx.Done():
				log.Print("Context done, exiting CanbusMonitor.Run")
				running = false
			}
			continue
		}

		select {
		case <-ctx.Done():
			log.Print("Context done, exiting CanbusMonitor.Run")
			running = false
			continue
		case cbm.C <- msg:
		case <-time.After(200 * time.Millisecond):
			log.Print("Timed-out sending frame through channel")
		default:

		}
		cbm.canbusdst.Send(msg)

	}

	cbm.canbussrc.Close()
	cbm.canbusdst.Close()

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
