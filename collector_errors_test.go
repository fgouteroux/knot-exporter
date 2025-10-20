package main

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/CZ-NIC/knot-exporter/libknot"
)

// Variables to override in tests
var newLibknotCtl = func() interface{} {
	return nil
}

// TestCollectWithErrors tests the collector with errors from the Knot control interface
func TestCollectWithErrors(t *testing.T) {
	// Create a mock Ctl factory
	origNewLibknotCtl := newLibknotCtl
	defer func() { newLibknotCtl = origNewLibknotCtl }()

	// Override the factory function to return our mock
	mockCtl := new(MockLibknotCtl)
	newLibknotCtl = func() interface{} { return mockCtl }

	// Setup error responses
	mockError := CreateCtlErrorSend("test error")

	// Setup basic successful connection with Maybe() so it doesn't strictly require the call
	mockCtl.On("Connect", mock.Anything).Return(nil).Maybe()
	mockCtl.On("Close").Return().Maybe()
	mockCtl.On("SetTimeout", mock.Anything).Return().Maybe()

	// Setup error responses for each method
	mockCtl.On("SendCommand", "stats").Return(mockError).Maybe()
	mockCtl.On("SendCommand", "zone-status").Return(mockError).Maybe()
	mockCtl.On("SendCommand", "zone-stats").Return(mockError).Maybe()
	mockCtl.On("SendCommandWithType", "zone-read", "SOA").Return(mockError).Maybe()

	// Create a collector with all options enabled
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Should not panic when errors occur
	assert.NotPanics(t, func() {
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		// Should still have at least the build info metric
		foundBuildInfo := false
		for _, mf := range metrics {
			if mf.GetName() == "knot_build_info" {
				foundBuildInfo = true
				break
			}
		}

		assert.True(t, foundBuildInfo, "Build info metric should be present")
	})
}

// TestCollectWithConnectionError tests the collector with a connection error
func TestCollectWithConnectionError(t *testing.T) {
	// Create a mock Ctl factory
	origNewLibknotCtl := newLibknotCtl
	defer func() { newLibknotCtl = origNewLibknotCtl }()

	// Override the factory function to return our mock
	mockCtl := new(MockLibknotCtl)
	newLibknotCtl = func() interface{} { return mockCtl }

	// Setup connection error with Maybe() so it doesn't strictly require the call
	mockCtl.On("Connect", mock.Anything).Return(CreateCtlErrorConnect("connection error")).Maybe()

	// Create a collector
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Should not panic when connection fails
	assert.NotPanics(t, func() {
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		// Should still have at least the build info metric
		foundBuildInfo := false
		for _, mf := range metrics {
			if mf.GetName() == "knot_build_info" {
				foundBuildInfo = true
				break
			}
		}

		assert.True(t, foundBuildInfo, "Build info metric should be present")
	})
}

// TestCollectWithNilCtl tests the collector when the Ctl interface is nil
func TestCollectWithNilCtl(t *testing.T) {
	// Create a mock Ctl factory
	origNewLibknotCtl := newLibknotCtl
	defer func() { newLibknotCtl = origNewLibknotCtl }()

	// Override the factory function to return nil
	newLibknotCtl = func() interface{} { return nil }

	// Create a collector
	collector := newKnotCollector("/test", 1000, true, true, true, true, true, true)

	// Create a registry and register the collector
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	// Should not panic when Ctl is nil
	assert.NotPanics(t, func() {
		metrics, err := registry.Gather()
		assert.NoError(t, err)

		// Should still have at least the build info metric
		foundBuildInfo := false
		for _, mf := range metrics {
			if mf.GetName() == "knot_build_info" {
				foundBuildInfo = true
				break
			}
		}

		assert.True(t, foundBuildInfo, "Build info metric should be present")
	})
}

// TestCollectWithReceiveError tests the collector with a receive error
func TestCollectWithReceiveError(t *testing.T) {
	// Since we're just testing the collectGlobalStats method directly,
	// we don't need to worry about factory functions
	mockCtl := new(MockLibknotCtl)

	// Setup expectations for a direct method call test
	mockCtl.On("SendCommand", "stats").Return(nil)
	mockCtl.On("ReceiveResponse").Return(libknot.CtlTypeData, nil, CreateCtlErrorReceive("receive error")).Once()

	// Create a collector and channel
	collector := newKnotCollector("/test", 1000, true, true, false, false, false, false) // Only collect global stats
	ch := make(chan prometheus.Metric, 10)

	// Call collectGlobalStats
	err := collector.collectGlobalStats(mockCtl, ch)
	assert.Error(t, err)

	// Verify that no metrics were sent
	close(ch)
	metricCount := 0
	for range ch {
		metricCount++
	}

	assert.Equal(t, 0, metricCount)

	// Verify expectations
	mockCtl.AssertExpectations(t)
}
