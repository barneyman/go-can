package canbus_native

import (
	"errors"
	"net"
	"testing"

	"github.com/go-daq/canbus"
)

func requireCANitf(t *testing.T, itf string) {
	t.Helper()
	iface, err := net.InterfaceByName(itf)
	if err != nil {
		t.Skipf("skipping test: %s interface not found (%v)", itf, err)
	}
	// Optional: also skip if the interface exists but is down
	if iface.Flags&net.FlagUp == 0 {
		t.Skipf("skipping test: %s interface is down", itf)
	}
}
func TestNew(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	if archive == nil {
		t.Fatalf("Expected native to be non-nil")
	}
}

func TestBindDefault(t *testing.T) {
	requireCANitf(t, "can0")
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	err = archive.BindDefault()
	if err != nil {
		t.Fatalf("Failed to bind default native: %v", err)
	}
	archive.Close()
}

func TestSendFailsNotBound(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	err = archive.Send(canbus.Frame{})
	if err == nil {
		t.Fatalf("Expected error when sending to unbound native")
	}
}

func TestRecvFailsNotBound(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	_, err = archive.Recv()
	if err == nil {
		t.Fatalf("Expected error when receiving from unbound native")
	}
}

func TestRecvTimesOut(t *testing.T) {
	requireCANitf(t, "vcan0")
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	err = archive.Bind("vcan0")
	if err != nil {
		t.Fatalf("Failed to bind default native: %v", err)
	}

	err = archive.SetRecvTimeout(1) // Set a short receive timeout for testing
	if err != nil {
		t.Fatalf("Failed to set receive timeout: %v", err)
	}

	_, err = archive.Recv()
	if err != nil {
		if errors.Is(err, canbus.ErrReceiveTimeout) {
			t.Logf("Receive timed out as expected")
		} else {
			t.Fatalf("Expected receive to timeout on native, got: %v", err)
		}
	}
	archive.Close()
}

func TestSendSucceeds(t *testing.T) {
	requireCANitf(t, "can0")
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new native: %v", err)
	}
	err = archive.BindDefault()
	if err != nil {
		t.Fatalf("Failed to bind default native: %v", err)
	}
	err = archive.Send(canbus.Frame{})
	if err != nil {
		t.Fatalf("Expected send to succeed on native, got error: %v", err)
	}
	archive.Close()
}

func TestNewVirtual(t *testing.T) {
	requireCANitf(t, "vcan0")
	archive, err := NewVirtual()
	if err != nil {
		t.Fatalf("Failed to create new virtual native: %v", err)
	}
	if archive == nil {
		t.Fatalf("Expected virtual native to be non-nil")
	}
}
