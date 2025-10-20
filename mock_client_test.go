package main

import (
	"github.com/CZ-NIC/knot-exporter/libknot"
	"github.com/stretchr/testify/mock"
)

// MockLibknotCtl is a mock for the KnotCtlInterface
type MockLibknotCtl struct {
	mock.Mock
}

func (m *MockLibknotCtl) Connect(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockLibknotCtl) Close() {
	m.Called()
}

func (m *MockLibknotCtl) SetTimeout(timeout int) {
	m.Called(timeout)
}

func (m *MockLibknotCtl) SendCommand(cmd string) error {
	args := m.Called(cmd)
	return args.Error(0)
}

func (m *MockLibknotCtl) SendCommandWithType(cmd string, rtype string) error {
	args := m.Called(cmd, rtype)
	return args.Error(0)
}

func (m *MockLibknotCtl) ReceiveResponse() (libknot.CtlType, *libknot.CtlData, error) {
	args := m.Called()
	dataType := args.Get(0).(libknot.CtlType)
	var data *libknot.CtlData
	if args.Get(1) != nil {
		data = args.Get(1).(*libknot.CtlData)
	}
	return dataType, data, args.Error(2)
}

// CreateCtlErrorSend creates a new CtlErrorSend error for testing
func CreateCtlErrorSend(message string) error {
	// We're using a custom error that mimics the behavior without accessing unexported fields
	return &TestCtlError{message: message, errorType: "send"}
}

// CreateCtlErrorConnect creates a new CtlErrorConnect error for testing
func CreateCtlErrorConnect(message string) error {
	return &TestCtlError{message: message, errorType: "connect"}
}

// CreateCtlErrorReceive creates a new CtlErrorReceive error for testing
func CreateCtlErrorReceive(message string) error {
	return &TestCtlError{message: message, errorType: "receive"}
}

// TestCtlError is a custom error type for tests that mimics libknot.CtlError
type TestCtlError struct {
	message   string
	errorType string
}

func (e *TestCtlError) Error() string {
	return e.message
}
