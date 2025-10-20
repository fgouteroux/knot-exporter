package main

import (
	"testing"

	"github.com/CZ-NIC/knot-exporter/libknot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestCollectGlobalStats tests the collectGlobalStats method
func TestCollectGlobalStats(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations for global stats
	mockCtl.On("SendCommand", "stats").Return(nil)

	// Setup some sample responses
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "server",
		Item:    "query.total",
		ID:      "udp",
		Data:    "1000",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "server",
		Item:    "query.total",
		ID:      "tcp",
		Data:    "500",
	}, nil).Once()

	// Signal end of data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectGlobalStats
	err := collector.collectGlobalStats(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 4 metrics (2 for each value - gauge and counter)
	assert.Equal(t, 4, metricCount)

	// Verify all expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectGlobalStats_InvalidData tests the collectGlobalStats method with invalid data
func TestCollectGlobalStats_InvalidData(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations for global stats
	mockCtl.On("SendCommand", "stats").Return(nil)

	// Setup some responses with invalid data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "server",
		Item:    "query.total",
		ID:      "udp",
		Data:    "not-a-number",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "server",
		Item:    "query.total",
		ID:      "tcp",
		Data:    "", // Empty data
	}, nil).Once()

	// Signal end of data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectGlobalStats - should not panic with invalid data
	assert.NotPanics(t, func() {
		err := collector.collectGlobalStats(mockCtl, ch)
		assert.NoError(t, err)
	})

	// Verify that no metrics were sent to the channel (all data was invalid)
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	assert.Equal(t, 0, metricCount, "Should not collect any metrics for invalid data")

	// Verify all expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectGlobalStats_Error tests the collectGlobalStats method with an error
func TestCollectGlobalStats_Error(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup error responses
	mockError := CreateCtlErrorSend("test error")

	mockCtl.On("SendCommand", "stats").Return(mockError)

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectGlobalStats - should return the error
	err := collector.collectGlobalStats(mockCtl, ch)
	assert.Error(t, err)

	// Verify all expectations
	mockCtl.AssertExpectations(t)
}
