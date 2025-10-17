package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/CZ-NIC/knot-exporter/libknot"
)

// Metric descriptors
var (
	memoryUsageDesc = makeDescPair(
		"knot_memory_usage_bytes",
		"Memory usage of Knot DNS processes",
		[]string{"pid"},
		nil,
	)

	zoneSerialDesc = makeDescPair(
		"knot_zone_serial",
		"Zone serial number from Knot DNS",
		[]string{"zone"},
		nil,
	)

	// Timer-specific metrics
	zoneRefreshDesc = makeDescPair(
		"knot_zone_refresh_seconds",
		"Zone SOA refresh timer",
		[]string{"zone"},
		nil,
	)

	zoneRetryDesc = makeDescPair(
		"knot_zone_retry_seconds",
		"Zone SOA retry timer",
		[]string{"zone"},
		nil,
	)

	zoneExpirationDesc = makeDescPair(
		"knot_zone_expiration_seconds",
		"Zone SOA expiration timer",
		[]string{"zone"},
		nil,
	)

	// Zone status timer metrics (from zone-status command)
	zoneStatusExpirationDesc = makeDescPair(
		"knot_zone_status_expiration_seconds",
		"Zone expiration timer from zone-status",
		[]string{"zone"},
		nil,
	)

	zoneStatusRefreshDesc = makeDescPair(
		"knot_zone_status_refresh_seconds",
		"Zone refresh timer from zone-status",
		[]string{"zone"},
		nil,
	)

	// Build info metric
	buildInfoDesc = prometheus.NewDesc(
		"knot_build_info",
		"Build information about the exporter and libknot",
		[]string{"version", "build_time", "git_commit", "go_version", "libknot_version", "platform"},
		nil,
	)
)

func makeDescPair(fqName, help string, variableLabels []string, constLabels prometheus.Labels) [2]*prometheus.Desc {
	return [2]*prometheus.Desc{
		prometheus.NewDesc(fqName, help, variableLabels, constLabels),
		prometheus.NewDesc(fqName+"_total", help, variableLabels, constLabels),
	}
}

// Dynamic metric descriptors for global stats and zone stats - will be created on demand
// 0th = base metric (gauge), 1st = %s_total metric (counter)
var (
	globalStatsDescriptors = make(map[string][2]*prometheus.Desc)
	globalStatsDescMutex   = sync.RWMutex{}

	zoneStatsDescriptors = make(map[string][2]*prometheus.Desc)
	zoneStatsDescMutex   = sync.RWMutex{}
)

func memoryUsage() map[string]uint64 {
	out := make(map[string]uint64)
	cmd := exec.Command("pidof", "knotd")
	output, err := cmd.Output()
	if err != nil {
		return out
	}
	pids := strings.Fields(string(output))
	for _, pidStr := range pids {
		if pid, err := strconv.Atoi(pidStr); err == nil {
			if usage := getProcessMemory(pid); usage > 0 {
				out[pidStr] = usage
			}
		}
	}
	return out
}

func getProcessMemory(pid int) uint64 {
	// Validate pid is reasonable
	if pid <= 0 || pid > 4194304 { // Max reasonable PID (4M)
		return 0
	}

	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0
	}

	// Search for VmRSS line in the content
	scanner := bufio.NewScanner(bytes.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if kb, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					return kb * 1024
				}
			}
			break
		}
	}

	return 0
}

// Get or create a metric descriptor for global stats
func getGlobalStatsDescriptor(item string) [2]*prometheus.Desc {
	globalStatsDescMutex.RLock()
	if desc, exists := globalStatsDescriptors[item]; exists {
		globalStatsDescMutex.RUnlock()
		return desc
	}
	globalStatsDescMutex.RUnlock()

	// Create new descriptor
	globalStatsDescMutex.Lock()
	defer globalStatsDescMutex.Unlock()

	// Double-check in case another goroutine created it
	if desc, exists := globalStatsDescriptors[item]; exists {
		return desc
	}

	// Create metric name based on item
	metricName := fmt.Sprintf("knot_stats_%s", sanitizeMetricName(item))

	// Create help text
	help := fmt.Sprintf("Global statistic: %s", item)

	// Create labels - always include section and type (using ID as type)
	labels := []string{"module", "type"}

	desc := makeDescPair(metricName, help, labels, nil)
	globalStatsDescriptors[item] = desc

	debugLog("Created new global stats descriptor: %s with labels: %v", metricName, labels)
	return desc
}

// Get or create a metric descriptor for zone stats
func getZoneStatsDescriptor(item string) [2]*prometheus.Desc {
	zoneStatsDescMutex.RLock()
	if desc, exists := zoneStatsDescriptors[item]; exists {
		zoneStatsDescMutex.RUnlock()
		return desc
	}
	zoneStatsDescMutex.RUnlock()

	// Create new descriptor
	zoneStatsDescMutex.Lock()
	defer zoneStatsDescMutex.Unlock()

	// Double-check in case another goroutine created it
	if desc, exists := zoneStatsDescriptors[item]; exists {
		return desc
	}

	// Create metric name based on item
	metricName := fmt.Sprintf("knot_zone_stats_%s", sanitizeMetricName(item))

	// Create help text
	help := fmt.Sprintf("Zone statistic: %s", item)

	// Create labels - always include zone, section and type (using ID as type)
	labels := []string{"zone", "module", "type"}

	desc := makeDescPair(metricName, help, labels, nil)
	zoneStatsDescriptors[item] = desc

	debugLog("Created new zone stats descriptor: %s with labels: %v", metricName, labels)
	return desc
}

// KnotCollector defines a collector for Knot DNS metrics
type KnotCollector struct {
	sockPath          string
	timeout           int
	collectMemInfo    bool
	collectStats      bool
	collectZoneStats  bool
	collectZoneStatus bool
	collectZoneTimers bool
	collectZoneSerial bool
	mu                sync.Mutex
	libknotVersion    string // Cache the libknot version
}

func newKnotCollector(sockPath string, timeout int,
	collectMemInfo, collectStats, collectZoneStats,
	collectZoneStatus, collectZoneSerial, collectZoneTimers bool) *KnotCollector {

	// Get libknot version once during initialization
	libknotVersion := libknot.GetVersion()

	return &KnotCollector{
		sockPath:          sockPath,
		timeout:           timeout,
		collectMemInfo:    collectMemInfo,
		collectStats:      collectStats,
		collectZoneStats:  collectZoneStats,
		collectZoneStatus: collectZoneStatus,
		collectZoneTimers: collectZoneTimers,
		collectZoneSerial: collectZoneSerial,
		libknotVersion:    libknotVersion,
	}
}

func (c *KnotCollector) convertStateTime(timeStr string) *float64 {
	// Check for special states
	if isPrefixIn(timeStr, []string{"pending", "running", "frozen"}) {
		result := float64(0)
		return &result
	}
	if timeStr == "not scheduled" || timeStr == "-" {
		return nil
	}

	// Parse time duration
	if seconds, ok := parseDurationString(timeStr); ok {
		return &seconds
	}

	log.Printf("error: unable to parse time string: %s", timeStr)

	return nil
}

// Describe implements prometheus.Collector interface
func (c *KnotCollector) Describe(ch chan<- *prometheus.Desc) {
	sendDesc := func(desc [2]*prometheus.Desc) {
		ch <- desc[0]
		ch <- desc[1]
	}

	// Always include build info
	ch <- buildInfoDesc

	if c.collectMemInfo {
		sendDesc(memoryUsageDesc)
	}

	// For global stats and zone stats, we can't pre-describe all metrics since they're dynamic
	// Prometheus will handle this automatically during collection

	if c.collectZoneSerial {
		sendDesc(zoneSerialDesc)
	}
	if c.collectZoneTimers {
		sendDesc(zoneRefreshDesc)
		sendDesc(zoneRetryDesc)
		sendDesc(zoneExpirationDesc)
		sendDesc(zoneStatusExpirationDesc)
		sendDesc(zoneStatusRefreshDesc)
	}
}

// send both the base metric (gauge) and its %s_total variant (counter)
func sendMetrics(ch chan<- prometheus.Metric, desc [2]*prometheus.Desc, value float64, labelValues ...string) {
	ch <- prometheus.MustNewConstMetric(
		desc[0],
		prometheus.GaugeValue,
		value,
		labelValues...,
	)
	ch <- prometheus.MustNewConstMetric(
		desc[1],
		prometheus.CounterValue,
		value,
		labelValues...,
	)
}

// Collect implements prometheus.Collector interface
func (c *KnotCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Always emit build info metric
	platform := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	ch <- prometheus.MustNewConstMetric(
		buildInfoDesc,
		prometheus.GaugeValue,
		1.0,
		version,
		buildTime,
		gitCommit,
		goVersion,
		c.libknotVersion,
		platform,
	)

	ctl := libknot.New()
	if ctl == nil {
		log.Printf("Failed to allocate knot control object")
		return
	}
	defer ctl.Close()

	err := ctl.Connect(c.sockPath)
	if err != nil {
		log.Printf("Failed to connect to socket: %v", err)
		return
	}
	ctl.SetTimeout(c.timeout)

	// Collect memory information
	if c.collectMemInfo {
		memUsage := memoryUsage()
		for pid, usage := range memUsage {
			sendMetrics(ch, memoryUsageDesc, float64(usage), pid)
		}
	}

	// Collect global statistics (only once per collection)
	if c.collectStats {
		if err := c.collectGlobalStats(ctl, ch); err != nil {
			log.Printf("Failed to collect global stats: %v", err)
		}
	}

	// We need a new connection for each command due to protocol limitations
	ctl.Close()
	ctl = libknot.New()
	if ctl == nil {
		return
	}
	defer ctl.Close()
	if err := ctl.Connect(c.sockPath); err != nil {
		return
	}
	ctl.SetTimeout(c.timeout)

	// Collect zone status (includes serials if enabled)
	if c.collectZoneStatus || c.collectZoneSerial {
		if err := c.collectZoneStatusInfo(ctl, ch); err != nil {
			log.Printf("Failed to collect zone status: %v", err)
		}
	}

	// Collect zone statistics if enabled
	if c.collectZoneStats {
		// Need another fresh connection
		ctl.Close()
		ctl = libknot.New()
		if ctl == nil {
			return
		}
		defer ctl.Close()
		if err := ctl.Connect(c.sockPath); err != nil {
			log.Printf("Failed to reconnect for zone stats: %v", err)
			return
		}
		ctl.SetTimeout(c.timeout)

		if err := c.collectZoneStatistics(ctl, ch); err != nil {
			log.Printf("Failed to collect zone stats: %v", err)
		}
	}

	// Collect zone timers if enabled
	if c.collectZoneTimers {
		// Need another fresh connection
		ctl.Close()
		ctl = libknot.New()
		if ctl == nil {
			return
		}
		defer ctl.Close()
		if err := ctl.Connect(c.sockPath); err != nil {
			log.Printf("Failed to reconnect for zone timers: %v", err)
			return
		}
		ctl.SetTimeout(c.timeout)

		if err := c.collectZoneTimerInfo(ctl, ch); err != nil {
			log.Printf("Failed to collect zone timers: %v", err)
		}
	}
}

// Helper methods for collecting different types of metrics
func (c *KnotCollector) collectGlobalStats(ctl *libknot.Ctl, ch chan<- prometheus.Metric) error {
	debugLog("Collecting global stats...")
	if err := ctl.SendCommand("stats"); err != nil {
		return err
	}

	count := 0
	responseCount := 0

	for {
		dataType, data, err := ctl.ReceiveResponse()
		if err != nil {
			return err
		}

		responseCount++

		// Debug every response for the first 20 responses
		if debugMode && responseCount <= 20 {
			debugLog("Response %d: type=%d, section='%s', item='%s', id='%s', zone='%s', data='%s'",
				responseCount, dataType, data.Section, data.Item, data.ID, data.Zone, data.Data)
		}

		// Break on BLOCK (end of response) or END (end of connection)
		if dataType == libknot.CtlTypeBlock || dataType == libknot.CtlTypeEnd {
			debugLog("Stats collection ended: type=%d, total responses=%d", dataType, responseCount)
			break
		}

		// Process both DATA (type=1) and EXTRA (type=2) responses
		if (dataType == libknot.CtlTypeData || dataType == libknot.CtlTypeExtra) && data.Item != "" && data.Data != "" {
			count++
			if value, err := strconv.ParseFloat(data.Data, 64); err == nil {
				debugLog("Global stat: section='%s', item='%s', id='%s', value=%s",
					data.Section, data.Item, data.ID, data.Data)

				// Get the dynamic metric descriptor
				desc := getGlobalStatsDescriptor(data.Item)
				sendMetrics(ch, desc, value,
					data.Section, // section label
					data.ID,      // type label (using ID field, can be empty)
				)
			} else {
				debugLog("Failed to parse value '%s' for item '%s'", data.Data, data.Item)
			}
		} else if dataType == libknot.CtlTypeData || dataType == libknot.CtlTypeExtra {
			// Debug cases where we skip metrics
			debugLog("Skipped metric: type=%d, item='%s', data='%s' (missing item or data)",
				dataType, data.Item, data.Data)
		}
	}

	debugLog("Global stats: collected %d statistics from %d total responses", count, responseCount)
	return nil
}

func (c *KnotCollector) collectZoneStatusInfo(ctl *libknot.Ctl, ch chan<- prometheus.Metric) error {
	debugLog("Collecting zone status...")
	if err := ctl.SendCommand("zone-status"); err != nil {
		return err
	}

	count := 0
	responseCount := 0
	currentZone := ""
	responseIndex := 0

	for {
		dataType, data, err := ctl.ReceiveResponse()
		if err != nil {
			return err
		}

		responseCount++
		if debugMode && responseCount <= 10 { // Debug first 10 records only in debug mode
			debugLog("Zone status response %d: type=%d, section='%s', item='%s', id='%s', zone='%s', data='%s'",
				responseCount, dataType, data.Section, data.Item, data.ID, data.Zone, data.Data)
		}

		// Break on BLOCK (end of response) or END (end of connection)
		if dataType == libknot.CtlTypeBlock || dataType == libknot.CtlTypeEnd {
			debugLog("Zone status collection complete, processed %d responses", responseCount)
			break
		}

		// Process both DATA (type=1) and EXTRA (type=2) responses
		if dataType == libknot.CtlTypeData || dataType == libknot.CtlTypeExtra {
			count++

			// Type 1 (DATA) with zone name indicates start of new zone
			if dataType == libknot.CtlTypeData && data.Zone != "" && data.Zone != currentZone {
				currentZone = data.Zone
				responseIndex = 0
			} else if dataType == libknot.CtlTypeExtra && currentZone != "" {
				// Type 2 (EXTRA) contains the zone details in order
				responseIndex++

				// Based on the output, position 1 appears to be the serial
				if c.collectZoneSerial && responseIndex == 1 {
					if serial, err := strconv.ParseFloat(data.Data, 64); err == nil {
						sendMetrics(ch, zoneSerialDesc, serial, currentZone)
					}
				}

				// Extract zone timer information from additional EXTRA responses
				if c.collectZoneStatus && data.Data != "" && data.Data != "-" {
					// Based on the actual zone-status output order after serial:
					// Position 7: refresh timer, Position 9: expiration timer
					switch responseIndex {
					case 7: // refresh timer (appears as +1h28m44s format)
						if seconds := c.convertStateTime(data.Data); seconds != nil {
							sendMetrics(ch, zoneStatusRefreshDesc, *seconds, currentZone)
							if debugMode {
								debugLog("Zone status refresh timer: zone=%s, position=%d, value=%s, seconds=%f",
									currentZone, responseIndex, data.Data, *seconds)
							}
						}
					case 9: // expiration timer (appears as +27D23h58m44s format)
						if seconds := c.convertStateTime(data.Data); seconds != nil {
							sendMetrics(ch, zoneStatusExpirationDesc, *seconds, currentZone)
							if debugMode {
								debugLog("Zone status expiration timer: zone=%s, position=%d, value=%s, seconds=%f",
									currentZone, responseIndex, data.Data, *seconds)
							}
						}
					}
				}
			}
		}
	}

	debugLog("Zone status: processed %d items from %d responses", count, responseCount)
	return nil
}

func (c *KnotCollector) collectZoneStatistics(ctl *libknot.Ctl, ch chan<- prometheus.Metric) error {
	debugLog("Collecting zone statistics...")
	if err := ctl.SendCommand("zone-stats"); err != nil {
		return err
	}

	count := 0
	responseCount := 0

	for {
		dataType, data, err := ctl.ReceiveResponse()
		if err != nil {
			return err
		}

		responseCount++
		if debugMode && responseCount <= 10 { // Debug first 10 responses only in debug mode
			debugLog("Zone stats response %d: type=%d, section='%s', item='%s', id='%s', zone='%s', data='%s'",
				responseCount, dataType, data.Section, data.Item, data.ID, data.Zone, data.Data)
		}

		// Break on BLOCK (end of response) or END (end of connection)
		if dataType == libknot.CtlTypeBlock || dataType == libknot.CtlTypeEnd {
			debugLog("Zone stats collection complete, processed %d responses", responseCount)
			break
		}

		// Process both DATA (type=1) and EXTRA (type=2) responses
		if (dataType == libknot.CtlTypeData || dataType == libknot.CtlTypeExtra) && data.Zone != "" && data.Item != "" && data.Data != "" {
			count++
			statType := data.Item
			statSubtype := data.ID

			if value, err := strconv.ParseFloat(data.Data, 64); err == nil {
				if debugMode && count <= 15 {
					debugLog("Zone stat: type=%d, zone=%s, section=%s, item=%s, id=%s, value=%s",
						dataType, data.Zone, data.Section, statType, statSubtype, data.Data)
				}

				// Get the dynamic metric descriptor
				desc := getZoneStatsDescriptor(statType)
				sendMetrics(ch, desc, value,
					data.Zone,    // zone label
					data.Section, // section label
					statSubtype,  // type label (using ID field)
				)
			} else {
				debugLog("Failed to parse zone stat value '%s' for zone '%s', item '%s'", data.Data, data.Zone, data.Item)
			}
		} else if dataType == libknot.CtlTypeData || dataType == libknot.CtlTypeExtra {
			// Debug cases where we skip metrics
			debugLog("Skipped zone stat: type=%d, zone='%s', item='%s', data='%s' (missing required fields)",
				dataType, data.Zone, data.Item, data.Data)
		}
	}

	debugLog("Zone stats: collected %d statistics from %d responses", count, responseCount)
	return nil
}

func (c *KnotCollector) collectZoneTimerInfo(ctl *libknot.Ctl, ch chan<- prometheus.Metric) error {
	debugLog("Collecting zone timers from SOA records...")

	// Use zone-read with SOA type to get only SOA records
	if err := ctl.SendCommandWithType("zone-read", "SOA"); err != nil {
		return fmt.Errorf("zone-read SOA command failed: %v", err)
	}

	count := 0
	maxResponses := 100000 // Limit responses

	for count < maxResponses {
		dataType, data, err := ctl.ReceiveResponse()
		if err != nil {
			return err
		}

		count++
		if debugMode && count <= 10 { // Debug first 10 records only in debug mode
			debugLog("Zone timer response %d: type=%d, zone='%s', data='%s'",
				count, dataType, data.Zone, data.Data)
		}

		// Break on BLOCK (end of response) or END (end of connection)
		if dataType == libknot.CtlTypeBlock || dataType == libknot.CtlTypeEnd {
			debugLog("Zone timers collection complete, processed %d responses", count)
			break
		}

		// Look for SOA records
		if dataType == libknot.CtlTypeData && data.Zone != "" {

			soaFields := strings.Fields(data.Data)
			if debugMode && count <= 5 {
				debugLog("Zone %s: parsed %d fields: %v", data.Zone, len(soaFields), soaFields)
			}

			// SOA format: "primary admin serial refresh retry expiration minimum"
			// Must have exactly 7 fields
			if len(soaFields) == 7 {
				// Check if this looks like a proper SOA record
				isPrimarySuffix := strings.HasSuffix(soaFields[0], ".")
				isAdminValid := strings.HasSuffix(soaFields[1], ".")

				if isPrimarySuffix && isAdminValid {
					// Check if fields 2-6 are numeric
					allNumeric := true
					var numericValues [5]int64

					for i := 2; i <= 6; i++ {
						val, err := strconv.ParseInt(soaFields[i], 10, 64)
						if err != nil {
							allNumeric = false
							break
						}
						numericValues[i-2] = val
					}

					if allNumeric {
						// Refresh timer (index 3 in SOA, index 1 in our array)
						sendMetrics(ch, zoneRefreshDesc, float64(numericValues[1]), data.Zone)

						// Retry timer (index 4 in SOA, index 2 in our array)
						sendMetrics(ch, zoneRetryDesc, float64(numericValues[2]), data.Zone)

						// Expiration timer (index 5 in SOA, index 3 in our array)
						sendMetrics(ch, zoneExpirationDesc, float64(numericValues[3]), data.Zone)
					} else {
						if debugMode && count <= 5 {
							debugLog("Zone %s: numeric validation failed", data.Zone)
						}
					}
				} else {
					if debugMode && count <= 5 {
						debugLog("Zone %s: format validation failed", data.Zone)
					}
				}
			} else {
				if debugMode && count <= 5 {
					debugLog("Zone %s: wrong field count (%d)", data.Zone, len(soaFields))
				}
			}
		}
	}

	if count >= maxResponses {
		debugLog("Zone timers: stopped at maximum responses (%d)", maxResponses)
	}

	debugLog("Zone timers: processed SOA records for %d zones", count)
	return nil
}
