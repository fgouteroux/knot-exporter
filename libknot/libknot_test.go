package libknot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGetVersion tests the GetVersion function
func TestGetVersion(t *testing.T) {
	// GetVersion returns a string
	version := GetVersion()
	assert.NotEmpty(t, version)
}

// TestCtlError tests the CtlError implementation
func TestCtlError(t *testing.T) {
	// Test CtlError without data
	err := &CtlError{message: "test error"}
	assert.Equal(t, "test error", err.Error())

	// Test CtlError with data
	errWithData := &CtlError{
		message: "test error with data",
		data: &CtlData{
			Section: "test-section",
			Item:    "test-item",
			Zone:    "example.com",
		},
	}
	assert.Contains(t, errWithData.Error(), "test error with data")
	assert.Contains(t, errWithData.Error(), "test-section")
	assert.Contains(t, errWithData.Error(), "example.com")

	// Test derived error types
	connectErr := &CtlErrorConnect{CtlError{message: "connection error"}}
	assert.Equal(t, "connection error", connectErr.Error())

	sendErr := &CtlErrorSend{CtlError{message: "send error"}}
	assert.Equal(t, "send error", sendErr.Error())

	receiveErr := &CtlErrorReceive{CtlError{message: "receive error"}}
	assert.Equal(t, "receive error", receiveErr.Error())

	remoteErr := &CtlErrorRemote{CtlError{message: "remote error"}}
	assert.Equal(t, "remote error", remoteErr.Error())
}

// TestCtlType tests the CtlType definitions
func TestCtlType(t *testing.T) {
	assert.Equal(t, CtlType(0), CtlTypeEnd)
	assert.Equal(t, CtlType(1), CtlTypeData)
	assert.Equal(t, CtlType(2), CtlTypeExtra)
	assert.Equal(t, CtlType(3), CtlTypeBlock)
}

// TestCtlData tests the CtlData structure
func TestCtlData(t *testing.T) {
	// Create a CtlData object
	data := CtlData{
		Section: "test-section",
		ID:      "test-id",
		Item:    "test-item",
		Zone:    "example.com",
		Data:    "test-data",
	}

	// Verify the data
	assert.Equal(t, "test-section", data.Section)
	assert.Equal(t, "test-id", data.ID)
	assert.Equal(t, "test-item", data.Item)
	assert.Equal(t, "example.com", data.Zone)
	assert.Equal(t, "test-data", data.Data)
}
