package main

import (
	"testing"

	"github.com/CZ-NIC/knot-exporter/libknot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// TestCollectZoneStatusInfo tests the collectZoneStatusInfo method
func TestCollectZoneStatusInfo(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommand", "zone-status").Return(nil)

	// Setup zone status responses
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.com",
	}, nil).Once()

	// Extra data for the zone (serial is position 1)
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
		Data: "2023101801", // Serial
	}, nil).Once()

	// More extra data to advance responseIndex
	for i := 0; i < 5; i++ {
		mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
			Data: "dummy",
		}, nil).Once()
	}

	// Refresh timer (position 7)
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
		Data: "+1h30m",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
		Data: "dummy",
	}, nil).Once()

	// Expiration timer (position 9)
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
		Data: "+30D",
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneStatusInfo
	err := collector.collectZoneStatusInfo(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 6 metrics (2 for each value - gauge and counter for serial, refresh, and expiration)
	assert.Equal(t, 6, metricCount)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneStatusInfo_InvalidData tests the collectZoneStatusInfo method with invalid data
func TestCollectZoneStatusInfo_InvalidData(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommand", "zone-status").Return(nil)

	// Setup zone status responses with invalid data
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Zone: "example.com",
	}, nil).Once()

	// Extra data with invalid serial
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeExtra, &libknot.CtlData{
		Data: "not-a-number", // Invalid serial
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, false, false, false, false, true, false) // Only collect serials
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneStatusInfo - should not panic with invalid data
	assert.NotPanics(t, func() {
		err := collector.collectZoneStatusInfo(mockCtl, ch)
		assert.NoError(t, err)
	})

	// Verify that no metrics were sent to the channel (all data was invalid)
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	assert.Equal(t, 0, metricCount, "Should not collect any metrics for invalid data")

	// Verify expectations
	mockCtl.AssertExpectations(t)
}

// TestCollectZoneStatistics tests the collectZoneStatistics method
func TestCollectZoneStatistics(t *testing.T) {
	// Create a mock Ctl
	mockCtl := new(MockLibknotCtl)

	// Setup expectations
	mockCtl.On("SendCommand", "zone-stats").Return(nil)

	// Setup zone stats responses for two zones
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "zone",
		Item:    "query.total",
		ID:      "",
		Zone:    "example.com",
		Data:    "1000",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "zone",
		Item:    "query.udp",
		ID:      "",
		Zone:    "example.com",
		Data:    "800",
	}, nil).Once()

	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, &libknot.CtlData{
		Section: "zone",
		Item:    "query.total",
		ID:      "",
		Zone:    "example.org",
		Data:    "500",
	}, nil).Once()

	// Signal end
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeBlock, nil, nil).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)
	ch := make(chan prometheus.Metric, 10)

	// Call collectZoneStatistics
	err := collector.collectZoneStatistics(mockCtl, ch)
	assert.NoError(t, err)

	// Verify that metrics were sent to the channel
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	// Should have 6 metrics (2 for each value - gauge and counter)
	assert.Equal(t, 6, metricCount)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}
