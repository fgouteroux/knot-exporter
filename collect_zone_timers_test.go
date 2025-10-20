package main

import (
	"testing"

	"github.com/CZ-NIC/knot-exporter/libknot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestCollectZoneTimerInfo tests the collectZoneTimerInfo method
func TestCollectZoneTimerInfo(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(nil)

	// Setup zone timer responses
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.com",
		Data: "ns1.example.com. admin.example.com. 2023101801 3600 600 86400 300",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.org",
		Data: "ns1.example.org. admin.example.org. 2023102501 7200 900 172800 600",
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 20)

	// Call collectZoneTimerInfo
	err := collector.collectZoneTimerInfo(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 12 metrics (2 zones × 3 timer types × 2 metric types (gauge/counter))
	assert.Equal(t, 12, metricCount)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneTimerInfo_InvalidSOA tests the collectZoneTimerInfo method with invalid SOA data
func TestCollectZoneTimerInfo_InvalidSOA(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(nil)

	// Setup zone timer responses with invalid SOA data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.com",
		Data: "invalid SOA data", // Too few fields
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.org",
		Data: "ns1.example.org admin.example.org 2023102501 invalid 900 172800 600", // Non-numeric field
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.net",
		Data: "ns1.example.net admin.example.net 2023102501 7200 900 172800 600 extra", // Too many fields
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneTimerInfo
	err := collector.collectZoneTimerInfo(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that no metrics were sent to the channel (all SOA data was invalid)
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	assert.Equal(t, 0, metricCount, "Should not collect any metrics for invalid SOA data")

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneTimerInfo_ValidAndInvalidSOA tests the collectZoneTimerInfo method with mixed SOA data
func TestCollectZoneTimerInfo_ValidAndInvalidSOA(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(nil)

	// Setup zone timer responses with mixed SOA data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.com",
		Data: "ns1.example.com. admin.example.com. 2023101801 3600 600 86400 300", // Valid SOA
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.org",
		Data: "invalid SOA data", // Invalid SOA
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneTimerInfo
	err := collector.collectZoneTimerInfo(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that metrics were sent to the channel only for the valid SOA
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 6 metrics (1 zone × 3 timer types × 2 metric types (gauge/counter))
	assert.Equal(t, 6, metricCount)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneTimerInfo_NoData tests the collectZoneTimerInfo method with no data
func TestCollectZoneTimerInfo_NoData(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(nil)

	// Signal end immediately
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneTimerInfo
	err := collector.collectZoneTimerInfo(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that no metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	assert.Equal(t, 0, metricCount, "Should not collect any metrics when no data is available")

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneTimerInfo_Error tests the collectZoneTimerInfo method with an error
func TestCollectZoneTimerInfo_Error(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup error responses
	mockError := CreateCtlErrorSend("test error")

	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(mockError)

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneTimerInfo - should return the error
	err := collector.collectZoneTimerInfo(mockCtl, ch)
	assert.Error(t, err)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}
