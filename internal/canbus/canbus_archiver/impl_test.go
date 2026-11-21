package canbus_archiver

import (
	"os"
	"testing"

	"github.com/go-daq/canbus"
)

func CreateTestData() {
	// create test archive file
	os.MkdirAll("./data", 0755)
}

func CleanupTestData() {
	os.RemoveAll("./data")
}

func TestNew(t *testing.T) {

	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	if archive == nil {
		t.Fatalf("Expected archive to be non-nil")
	}
}

func TestBindDefault(t *testing.T) {
	CreateTestData()
	defer CleanupTestData()

	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	err = archive.BindDefault()
	if err != nil {
		t.Fatalf("Failed to bind default archive: %v", err)
	}
	// clean up by closing and deleting the archive
	archive.Close()
}

func TestBind(t *testing.T) {
	CreateTestData()
	defer CleanupTestData()

	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	err = archive.Bind("./data/test_archive")
	if err != nil {
		t.Fatalf("Failed to bind archive: %v", err)
	}

	// clean up by closing and deleting the archive
	archive.Close()
}

func TestRecvFails(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	_, err = archive.Recv()
	if err == nil {
		t.Fatalf("Expected error when receiving from unbound archive")
	}
}

func TestSendFailsNotBound(t *testing.T) {
	CreateTestData()
	defer CleanupTestData()

	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	err = archive.Send(canbus.Frame{})
	if err == nil {
		t.Fatalf("Expected error when sending to unbound archive")
	}
}

func TestSendSucceeds(t *testing.T) {
	CreateTestData()
	defer CleanupTestData()

	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	err = archive.Bind("./data/test_archive")
	if err != nil {
		t.Fatalf("Failed to bind archive: %v", err)
	}

	err = archive.Send(canbus.Frame{})
	if err != nil {
		t.Fatalf("Expected send to succeed, got error: %v", err)
	}

	archive.Close()
}
