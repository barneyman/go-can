package canbus_archive

import (
	"os"
	"testing"

	"github.com/go-daq/canbus"
)

func TestNew(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	if archive == nil {
		t.Fatalf("Expected archive to be non-nil")
	}
}

func CreateTestData() {
	// create test archive file
	os.MkdirAll("./data", 0755)
	os.WriteFile("./data/test_archive", []byte("test data"), 0644)
}

func CleanupTestData() {
	os.Remove("./data/test_archive")
	os.RemoveAll("./data")
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

func TestRecvNoArchiveBound(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	_, err = archive.Recv()
	if err == nil {
		t.Fatalf("Expected error when receiving from unbound archive")
	}
}

func TestSend(t *testing.T) {
	archive, err := New()
	if err != nil {
		t.Fatalf("Failed to create new archive: %v", err)
	}
	err = archive.Send(canbus.Frame{})
	if err != nil {
		t.Fatalf("Expected no error when sending to archive: %v", err)
	}
}
