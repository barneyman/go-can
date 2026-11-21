package PID

import (
	"context"
	"encoding/hex"
	"errors"
	canbusmon "go-can/internal/canbus"
	"log"
	"sync"
	"time"

	"github.com/go-daq/canbus"
)

///////////////////////////////////////////////

type PIDsupported int
type PIDscanning int

const (
	UnknownSupport PIDsupported = iota
	Supported
	Unsupported
)

const (
	NotStarted PIDscanning = iota
	Ongoing
	Complete
	Closed
)

type PIDHandler struct {
	suppportedPIDs [255]PIDsupported
	scanStarted    PIDscanning
	sendPids       bool
	cb             *canbusmon.CanbusMonitor
	pidDecoders    []PidResultDecoder
}

type PIDresponse struct {
	respID  int
	service uint8
	PID     int
	data    []byte
}

// Arbitration IDs

const OBD2_ID uint16 = 0x7DF
const OBD2_ID_MAX uint32 = 0x7E7
const OBD2_ID_RESPONSE uint32 = 0x7E8
const OBD2_ID_MAX_RESPONSE uint32 = 0x7EF

// Flags
const PID_RESPONSE_FLAG byte = 0x40

// PIDs
const PID_RPM int = 0xC
const PID_SPEED_KPH int = 0xD

// services
const ODBC_SERVICE_CURRENT byte = 0x01

func NewPIDHandler() *PIDHandler {
	return &PIDHandler{}
}

func (ph *PIDHandler) Init(cb *canbusmon.CanbusMonitor) error {
	// Initialize the PIDHandler with the canbus monitor
	ph.cb = cb
	ph.pidDecoders = PidDefaultDecoders
	return nil
}

// launched as a go routine
func (ph *PIDHandler) Run(ctx context.Context, wg *sync.WaitGroup) {

	// add my interest
	wg.Add(1)
	defer wg.Done()

	// kick off the PID scanning process
	go func() {
		ph.scanStarted = Ongoing

		for supported := uint(0); supported < 8; supported++ {
			// get the PID supported by the vehicle
			reqFrame, err := ph.GetSupportedPIDRequestFrame(supported)
			if err != nil {
				log.Printf("Error getting supported PID request frame: %v", err)
				continue
			}
			// send it, the vehicle should respond with supported PIDs
			// handled by the channel listener below
			if err := ph.cb.Send(reqFrame); err != nil {
				log.Printf("Error sending supported PID request frame: %v", err)
				continue
			}
			// sleep for a little to allow the vehicle to respond
			time.Sleep(100 * time.Millisecond)
		}

		ph.scanStarted = Complete
	}()

	for running := true; running; {
		select {
		case <-ctx.Done():
			running = false
		case frame := <-ph.cb.C:
			ph.ParseFrame(frame)
		}
	}
	log.Print("PIDHandler.Run exiting")
}

func (ph *PIDHandler) ParseFrame(canbusFrame canbus.Frame) (bool, error) {

	isResponsePid := false

	if canbusFrame.ID < uint32(OBD2_ID_RESPONSE) || canbusFrame.ID > uint32(OBD2_ID_MAX_RESPONSE) {
		// it's not a PID, but it might be a CAN message, so don't error out, just ignore it
		log.Print("Not a PID response, ignoring")
		log.Printf("ArbitrationID: %x Kind: %v, Raw Data: %v", canbusFrame.ID, canbusFrame.Kind, hex.EncodeToString(canbusFrame.Data))
		return isResponsePid, nil
	}

	// check packet integrity
	pidlen := int(canbusFrame.Data[0])
	// PIDdata is padded (normally to 8 bytes), so the length of the data should be at least 3 (len, service, PID) + pidlen
	if len(canbusFrame.Data) <= pidlen {
		return isResponsePid, errors.New("wrong size payload")
	}

	// confirm it's a response
	if canbusFrame.Data[1]&PID_RESPONSE_FLAG == 0 {
		return isResponsePid, errors.New("not a response payload, flag not set")
	}

	isResponsePid = true
	response := PIDresponse{respID: int(canbusFrame.ID), service: uint8(canbusFrame.Data[1] &^ PID_RESPONSE_FLAG), PID: int(canbusFrame.Data[2]), data: canbusFrame.Data[3:]}

	// handle supported PID response
	if canbusFrame.Data[2]%0x20 == 0 {
		// this is a supported PID response
		// populate the array
		offset := 1 + (response.PID)

		for _, v := range response.data {
			for bit := 7; bit >= 0; bit-- {
				if (v & (1 << bit)) != 0 {
					ph.suppportedPIDs[offset] = Supported
				} else {
					ph.suppportedPIDs[offset] = Unsupported
				}
				offset++
			}
		}
	} else {

		// can we decode this PID using the registered decoders?
		for _, decoder := range ph.pidDecoders {
			if decoder.PID == response.PID {
				if err := decoder.Validate(response); err != nil {
					log.Printf("Validation failed for PID %d: %v", response.PID, err)
					break
				}
				decoded, err := decoder.Decode(response)
				if err != nil {
					log.Printf("Decoding failed for PID %d: %v", response.PID, err)
					break
				}
				log.Printf("Decoded PID %d: %s", response.PID, decoded)
				break
			}
		}
	}

	return isResponsePid, nil
}

func (ph *PIDHandler) Close() error {
	// perform any cleanup if necessary
	return nil
}

/////////////////////////////////////////////////////

// show ODBC_SERVICE_CURRENT
func (ph PIDHandler) GetOBD_PID_CurrentData(PID uint32, reqData []byte) (canbus.Frame, error) {

	return ph.GetPIDRequest(OBD2_ID, ODBC_SERVICE_CURRENT, PID, reqData)
}

// func (ph PIDHandler) sendOBD_PID(Service uint8, PID uint32, reqData []byte) error {

// 	return ph.sendPIDRequest(OBD2_ID, Service, PID, reqData)
// }

func (ph PIDHandler) GetPIDRequest(ArbitrationID uint16, Service uint8, PID uint32, reqData []byte) (canbus.Frame, error) {

	log.Printf("Build PIDRequest: ArbitrationID: %x Service: %d PID: %x\r", ArbitrationID, Service, PID)

	// check
	if ph.suppportedPIDs[PID] == Unsupported {
		return canbus.Frame{}, errors.New("unsupported pid")
	}

	frameData := []byte{byte(Service), byte(PID)}
	frameData = append(frameData, reqData...)

	// pad out to 8
	currentLen := len(frameData)
	if currentLen < 8 {
		add := make([]byte, 8-currentLen)
		frameData = append(frameData, add...)
	}

	cf := canbus.Frame{ID: uint32(ArbitrationID), Data: frameData, Kind: canbus.SFF}

	return cf, nil

}

func (ph *PIDHandler) GetSupportedPIDRequestFrame(startAt uint) (canbus.Frame, error) {

	// if ph.SendPids == false {
	// 	// just fake 'no PIDs supported' and stop the loop
	// 	ph.ScanStarted = Complete
	// 	log.Printf("Listening only, not sending PIDs\r")
	// 	return nil
	// }

	log.Printf("Build GetSupportedPID %d\r", startAt)

	// check startAt
	if startAt > 7 {
		return canbus.Frame{}, errors.New("PID request too high")
	}

	return ph.GetOBD_PID_CurrentData(uint32(startAt*0x20), []byte{})
}

//////////////////////////////////////////////////////////////

// func (ph PIDHandler) GetRPM() error {
// 	return ph.sendOBD_PID_CurrentData(uint32(PID_RPM), []byte{})
// }

// func (ph PIDHandler) GetSpeedKPH() error {
// 	return ph.sendOBD_PID_CurrentData(uint32(PID_SPEED_KPH), []byte{})
// }

// func (ph PIDHandler) GetThrottlePos() error {
// 	return ph.sendOBD_PID_CurrentData(0x11, []byte{})
// }

// func (ph PIDHandler) GetEngineRuntimeS() error {
// 	return ph.sendOBD_PID_CurrentData(0x1F, []byte{})
// }

// func (ph PIDHandler) GetFuelLevel() error {
// 	return ph.sendOBD_PID_CurrentData(0x2F, []byte{})
// }
