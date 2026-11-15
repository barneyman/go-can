package odb_handler

import (
	"errors"
	"log"

	"github.com/go-daq/canbus"
)

///////////////////////////////////////////////

type CanbusHandler interface {
	BindDefault() error
	SetListenOnly(bool)
	Bind(string) error
	Close()
	Recv() (canbus.Frame, error)
	Send(canbus.Frame) error
}

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
	CbHandler      CanbusHandler
	suppportedPIDs [255]PIDsupported
	ScanStarted    PIDscanning
	SendPids       bool
}

type PIDresponse struct {
	respID  int
	service int
	PID     int
	data    []byte
}

const PID_RESPONSE_FLAG byte = 0x40
const OBD2_ID uint = 0x7DF

func (ph *PIDHandler) Parse(resp canbus.Frame) error {

	// check packet integrity
	pidlen := int(resp.Data[0])

	if len(resp.Data) <= pidlen {
		return errors.New("wrong size payload")
	}

	// confirm it's a response
	if resp.Data[1]&0x40 == 0 {
		return errors.New("not a response payload")
	}

	response := PIDresponse{respID: int(resp.ID), service: int(resp.Data[1] &^ PID_RESPONSE_FLAG), PID: int(resp.Data[2]), data: resp.Data[3:]}

	log.Printf("Parsing ID: %x Service %d PID: %x\r", response.respID, response.service, response.PID)

	if response.service != 1 {
		return errors.New("unsupported service")
	}

	switch response.PID {
	// handle supported PIDs
	case 0, 32, 64, 96, 128, 160, 192:
		if response.PID == 0 {
			ph.ScanStarted = Ongoing
		}
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

		if response.PID < 192 {
			ph.GetSupportedPID((uint(response.PID) / 0x20) + 1)
		} else {
			ph.ScanStarted = Complete
		}
		return nil
	}

	return nil
}

func (ph PIDHandler) sendOBD_PID_Current(PID uint, reqData []byte) error {

	return ph.sendPID(OBD2_ID, 1, PID, reqData)
}

func (ph PIDHandler) sendOBD_PID(Service, PID uint, reqData []byte) error {

	return ph.sendPID(OBD2_ID, Service, PID, reqData)
}

func (ph PIDHandler) sendPID(ID, Service, PID uint, reqData []byte) error {

	// check
	if ph.suppportedPIDs[PID] == Unsupported {
		return errors.New("unsupported pid")
	}

	frameData := []byte{byte(Service), byte(PID)}
	frameData = append(frameData, reqData...)

	// pad out to 8
	currentLen := len(frameData)
	if currentLen < 8 {
		add := make([]byte, 8-currentLen)
		frameData = append(frameData, add...)
	}

	cf := canbus.Frame{ID: uint32(ID), Data: frameData, Kind: canbus.SFF}

	return ph.CbHandler.Send(cf)

}

func (ph *PIDHandler) GetSupportedPID(startAt uint) error {

	if ph.SendPids == false {
		// just fake 'no PIDs supported' and stop the loop
		ph.ScanStarted = Complete
		log.Printf("Listening only, not sending PIDs\r")
		return nil
	}

	log.Printf("GetSupportedPID %d\r", startAt)

	// check startAt
	if startAt > 7 {
		return errors.New("PID request too high")
	}

	return ph.sendOBD_PID_Current(startAt*0x20, []byte{})
}

//////////////////////////////////////////////////////////////

func (ph PIDHandler) GetRPM() error {
	return ph.sendOBD_PID_Current(0xC, []byte{})
}

func (ph PIDHandler) GetSpeedKPH() error {
	return ph.sendOBD_PID_Current(0xD, []byte{})
}

func (ph PIDHandler) GetThrottlePos() error {
	return ph.sendOBD_PID_Current(0x11, []byte{})
}

func (ph PIDHandler) GetEngineRuntimeS() error {
	return ph.sendOBD_PID_Current(0x1F, []byte{})
}

func (ph PIDHandler) GetFuelLevel() error {
	return ph.sendOBD_PID_Current(0x2F, []byte{})
}
