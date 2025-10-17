package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/CZ-NIC/knot-exporter/libknot"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Build information - set via build flags
var (
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
	goVersion = runtime.Version()
)

// Global debug flag
var debugMode bool

// Version information
func printVersion() {
	libknotVersion := libknot.GetVersion()
	fmt.Printf("Knot DNS Exporter\n")
	fmt.Printf("  Version:      %s\n", version)
	fmt.Printf("  Build time:   %s\n", buildTime)
	fmt.Printf("  Git commit:   %s\n", gitCommit)
	fmt.Printf("  Go version:   %s\n", goVersion)
	fmt.Printf("  Libknot:      %s\n", libknotVersion)
	fmt.Printf("  Platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
}

// validateConfig performs basic validation of configuration
func validateConfig(sockPath string, addr string, port int) error {
	// Check if socket path exists and is accessible
	if _, err := os.Stat(sockPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("knot socket does not exist: %s (is Knot DNS running?)", sockPath)
		}
		return fmt.Errorf("cannot access knot socket %s: %v", sockPath, err)
	}

	// Validate network address
	if net.ParseIP(addr) == nil && addr != "localhost" {
		return fmt.Errorf("invalid listen address: %s", addr)
	}

	// Validate port range
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number: %d (must be 1-65535)", port)
	}

	// Check if port is available
	listener, err := net.Listen("tcp", net.JoinHostPort(addr, strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("cannot bind to %s:%d: %v", addr, port, err)
	}
	if err := listener.Close(); err != nil {
		return fmt.Errorf("error closing listener: %v", err)
	}

	return nil
}

// testKnotConnection tests if we can connect to Knot DNS
func testKnotConnection(sockPath string, timeout int) error {
	debugLog("Testing connection to Knot DNS at %s", sockPath)

	ctl := libknot.New()
	if ctl == nil {
		return fmt.Errorf("failed to allocate knot control object")
	}
	defer ctl.Close()

	ctl.SetTimeout(timeout)
	if err := ctl.Connect(sockPath); err != nil {
		return fmt.Errorf("failed to connect to knot socket: %v", err)
	}

	// Test a simple command
	if err := ctl.SendCommand("status"); err != nil {
		return fmt.Errorf("failed to send test command to knot: %v", err)
	}

	// Try to read at least one response
	_, _, err := ctl.ReceiveResponse()
	if err != nil {
		return fmt.Errorf("failed to receive response from knot: %v", err)
	}

	debugLog("Successfully connected to Knot DNS")
	return nil
}

// setupGracefulShutdown sets up graceful shutdown handling
func setupGracefulShutdown(server *http.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)

		// Create a context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Attempt graceful shutdown
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Error during shutdown: %v", err)
			os.Exit(1)
		}

		log.Printf("Server stopped gracefully")
		os.Exit(0)
	}()
}

// healthCheck provides a basic health check endpoint
func healthCheck(sockPath string, timeout int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := testKnotConnection(sockPath, timeout); err != nil {
			http.Error(w, fmt.Sprintf("Health check failed: %v", err), http.StatusServiceUnavailable)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := fmt.Fprintf(w, "OK")
		if err != nil {
			log.Printf("Error writing health check response: %v", err)
		}
	}
}

func main() {
	webListenAddr := flag.String("web-listen-addr", "127.0.0.1", "address on which to expose metrics")
	webListenPort := flag.Int("web-listen-port", 9433, "port on which to expose metrics")
	knotSocketPath := flag.String("knot-socket-path", "/run/knot/knot.sock", "path to knot control socket")
	knotSocketTimeout := flag.Int("knot-socket-timeout", 2000, "timeout for Knot control socket operations")
	noMeminfo := flag.Bool("no-meminfo", false, "disable collection of memory usage")
	noGlobalStats := flag.Bool("no-global-stats", false, "disable collection of global statistics")
	noZoneStats := flag.Bool("no-zone-stats", false, "disable collection of zone statistics")
	noZoneStatus := flag.Bool("no-zone-status", false, "disable collection of zone status")
	noZoneSerial := flag.Bool("no-zone-serial", false, "disable collection of zone serial")
	zoneTimers := flag.Bool("zone-timers", false, "enables collection of zone SOA timer values")
	debug := flag.Bool("debug", false, "enable debug logging")
	showVersion := flag.Bool("version", false, "show version information and exit")
	skipValidation := flag.Bool("skip-validation", false, "skip initial validation checks (useful for testing)")

	flag.Parse()

	// Set global debug flag
	debugMode = *debug

	// Show version and exit
	if *showVersion {
		printVersion()
		os.Exit(0)
	}

	log.Printf("Starting Knot DNS Exporter %s", version)
	if debugMode {
		log.Printf("Debug mode enabled")
	}

	// Validate configuration unless skipped
	if !*skipValidation {
		log.Printf("Validating configuration...")
		if err := validateConfig(*knotSocketPath, *webListenAddr, *webListenPort); err != nil {
			log.Fatalf("Configuration validation failed: %v", err)
		}

		// Test Knot connection
		log.Printf("Testing connection to Knot DNS...")
		if err := testKnotConnection(*knotSocketPath, *knotSocketTimeout); err != nil {
			log.Fatalf("Knot DNS connection test failed: %v", err)
		}
		log.Printf("Configuration validation passed")
	} else {
		log.Printf("Skipping validation checks")
	}

	// Create collector with error handling
	log.Printf("Initializing metrics collector...")
	collector := newKnotCollector(
		*knotSocketPath,
		*knotSocketTimeout,
		!*noMeminfo,
		!*noGlobalStats,
		!*noZoneStats,
		!*noZoneStatus,
		!*noZoneSerial,
		*zoneTimers,
	)

	// Register collector with Prometheus
	if err := prometheus.Register(collector); err != nil {
		log.Fatalf("Failed to register Prometheus collector: %v", err)
	}

	// Setup HTTP routes
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthCheck(*knotSocketPath, *knotSocketTimeout))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Knot DNS Exporter</title></head>
<body>
<h1>Knot DNS Exporter</h1>
<p>Version: %s</p>
<p><a href="/metrics">Metrics</a></p>
<p><a href="/health">Health Check</a></p>
</body>
</html>`, version)
		if err != nil {
			log.Printf("Error writing index page: %v", err)
		}
	})

	// Create server with timeouts
	server := &http.Server{
		Addr:         net.JoinHostPort(*webListenAddr, strconv.Itoa(*webListenPort)),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Setup graceful shutdown
	setupGracefulShutdown(server)

	log.Printf("Starting HTTP server on %s", server.Addr)
	log.Printf("Metrics available at http://%s/metrics", server.Addr)
	log.Printf("Health check available at http://%s/health", server.Addr)

	// Start server with error handling
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
