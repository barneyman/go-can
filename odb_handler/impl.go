package odb_handler

import (
	"encoding/hex"
	"errors"
	"fmt"
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
	service uint8
	PID     int
	data    []byte
}

type PidResultDecoder struct {
	PID      int16
	Validate func(frame PIDresponse) error
	Decode   func(frame PIDresponse) (string, error)
}

// Arbitration IDs

const OBD2_ID uint16 = 0x7DF
const OBD2_ID_MAX uint32 = 0x7E7
const OBD2_ID_RESPONSE uint32 = 0x7E8
const OBD2_ID_MAX_RESPONSE uint32 = 0x7EF

// Flags
const PID_RESPONSE_FLAG byte = 0x40

// PIDs
const PID_RPM int16 = 0xC
const PID_SPEED_KPH int16 = 0xD

// services
const ODBC_SERVICE_CURRENT byte = 0x01

var PidResultDecoders = []PidResultDecoder{
	{PID: PID_RPM,
		Validate: func(frame PIDresponse) error {
			if len(frame.data) < 2 {
				return errors.New("not enough data for RPM")
			}
			return nil
		},
		Decode: func(frame PIDresponse) (string, error) {
			if len(frame.data) < 2 {
				return "", errors.New("not enough data for RPM")
			}
			rpm := (int(frame.data[0])<<8 | int(frame.data[1])) / 4
			return fmt.Sprintf("%d RPM", rpm), nil
		},
	},
	{PID: PID_SPEED_KPH,
		Validate: func(frame PIDresponse) error {
			if len(frame.data) < 1 {
				return errors.New("not enough data for speed")
			}
			return nil
		},
		Decode: func(frame PIDresponse) (string, error) {
			if len(frame.data) < 1 {
				return "", errors.New("not enough data for speed")
			}
			speed := int(frame.data[0])
			return fmt.Sprintf("%d KPH", speed), nil
		},
	},
}

func (ph *PIDHandler) Parse(resp canbus.Frame) (bool, error) {

	log.Print("PIDHandler.Parse()")
	defer log.Print("PIDHandler.Parse() done")

	isResponsePid := false

	// check its ID
	// TODO: nail this down
	if resp.ID < uint32(OBD2_ID_RESPONSE) || resp.ID > uint32(OBD2_ID_MAX_RESPONSE) {
		// it's not a PID, but it might be a CAN message, so don't error out, just ignore it
		log.Print("Not a PID response, ignoring")
		log.Printf("ArbitrationID: %x Kind: %v, Raw Data: %v", resp.ID, resp.Kind, hex.EncodeToString(resp.Data))
		return isResponsePid, nil
	}

	// check packet integrity
	pidlen := int(resp.Data[0])
	// PIDdata is padded (normally to 8 bytes), so the length of the data should be at least 3 (len, service, PID) + pidlen
	if len(resp.Data) <= pidlen {
		return isResponsePid, errors.New("wrong size payload")
	}

	// confirm it's a response
	if resp.Data[1]&PID_RESPONSE_FLAG == 0 {
		return isResponsePid, errors.New("not a response payload, flag not set")
	}

	isResponsePid = true
	response := PIDresponse{respID: int(resp.ID), service: uint8(resp.Data[1] &^ PID_RESPONSE_FLAG), PID: int(resp.Data[2]), data: resp.Data[3:]}

	log.Printf("Parsing ID: %x Service %d PID: %x\r", response.respID, response.service, response.PID)

	if response.service != ODBC_SERVICE_CURRENT {
		return isResponsePid, errors.New("unsupported service")
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
		return isResponsePid, nil
	case int(PID_RPM), int(PID_SPEED_KPH):
		for _, decoder := range PidResultDecoders {
			if decoder.PID == int16(response.PID) {
				err := decoder.Validate(response)
				if err != nil {
					return isResponsePid, err
				}
				result, err := decoder.Decode(response)
				if err != nil {
					return isResponsePid, err
				}
				log.Printf("Decoded result for PID %d: %s", response.PID, result)
			}
		}
	}

	return isResponsePid, nil
}

// show ODBC_SERVICE_CURRENT
func (ph PIDHandler) sendOBD_PID_CurrentData(PID uint32, reqData []byte) error {

	return ph.sendPIDRequest(OBD2_ID, ODBC_SERVICE_CURRENT, PID, reqData)
}

func (ph PIDHandler) sendOBD_PID(Service uint8, PID uint32, reqData []byte) error {

	return ph.sendPIDRequest(OBD2_ID, Service, PID, reqData)
}

func (ph PIDHandler) sendPIDRequest(ArbitrationID uint16, Service uint8, PID uint32, reqData []byte) error {

	log.Printf("sendPIDRequest: ArbitrationID: %x Service: %d PID: %x\r", ArbitrationID, Service, PID)
	defer log.Print("sendPIDRequest done")

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

	cf := canbus.Frame{ID: uint32(ArbitrationID), Data: frameData, Kind: canbus.SFF}

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

	return ph.sendOBD_PID_CurrentData(uint32(startAt*0x20), []byte{})
}

//////////////////////////////////////////////////////////////

func (ph PIDHandler) GetRPM() error {
	return ph.sendOBD_PID_CurrentData(uint32(PID_RPM), []byte{})
}

func (ph PIDHandler) GetSpeedKPH() error {
	return ph.sendOBD_PID_CurrentData(uint32(PID_SPEED_KPH), []byte{})
}

func (ph PIDHandler) GetThrottlePos() error {
	return ph.sendOBD_PID_CurrentData(0x11, []byte{})
}

func (ph PIDHandler) GetEngineRuntimeS() error {
	return ph.sendOBD_PID_CurrentData(0x1F, []byte{})
}

func (ph PIDHandler) GetFuelLevel() error {
	return ph.sendOBD_PID_CurrentData(0x2F, []byte{})
}
