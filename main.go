package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Config holds the application configuration
type Config struct {
	Interval        int    // Check interval in seconds
	LogFile         string // Path to log file
	IPEndpoint      string // Endpoint to check IP from
	QuietMode       bool   // If true, only logs to file and not stdout
	MaxRetries      int    // Maximum number of retry attempts for IP fetching
	LastKnownIP     string // Cache the last known IP
	LastKnownIPLock sync.Mutex
}

func main() {
	// Set up logging
	log.SetFlags(log.Ldate | log.Ltime)

	// Parse command-line flags
	config := parseFlags()

	// Create log file if specified
	setupLogging(config)

	// Start the IP checking loop
	logMessage("IP Watcher starting. Will check IP every %d seconds", config.Interval)
	startIPChecker(config)
}

// parseFlags parses command-line flags and returns the configuration
func parseFlags() *Config {
	config := &Config{}

	flag.IntVar(&config.Interval, "interval", 60, "Interval between IP checks in seconds")
	flag.StringVar(&config.LogFile, "log", "", "Log file path (if not specified, logs to stdout only)")
	flag.StringVar(&config.IPEndpoint, "endpoint", "https://api64.ipify.org?format=json", "URL of the IP checking service")
	flag.BoolVar(&config.QuietMode, "quiet", false, "If true, only logs to file and not stdout")
	flag.IntVar(&config.MaxRetries, "max-retries", 5, "Maximum number of retry attempts when fetching external IP")

	// Parse flags
	flag.Parse()

	return config
}

// setupLogging configures logging to file if a log file is specified
func setupLogging(config *Config) {
	if config.LogFile != "" {
		// Create directory for log file if it doesn't exist
		logDir := filepath.Dir(config.LogFile)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("Failed to create log directory: %v", err)
			os.Exit(1)
		}

		// Open log file
		logFile, err := os.OpenFile(config.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file: %v", err)
			os.Exit(1)
		}

		// Set log output to the file
		if config.QuietMode {
			log.SetOutput(logFile)
		} else {
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		}
	}
}

// logMessage logs a message to the configured output
func logMessage(format string, args ...any) {
	log.Printf(format, args...)
}

// startIPChecker starts the main loop checking for IP changes
func startIPChecker(config *Config) {
	ticker := time.NewTicker(time.Duration(config.Interval) * time.Second)
	defer ticker.Stop()

	// Do an initial IP check
	checkIP(config)

	// Then check according to the timer
	for range ticker.C {
		checkIP(config)
	}
}

// checkIP performs a single IP check
func checkIP(config *Config) {
	// Make HTTP request to get IP
	ip, err := fetchExternalIP(config.IPEndpoint, 1, config.MaxRetries)
	if err != nil {
		logMessage("Error checking IP: %v", err)
		return
	}

	// Compare with last known IP
	config.LastKnownIPLock.Lock()
	defer config.LastKnownIPLock.Unlock()

	if ip != config.LastKnownIP {
		if config.LastKnownIP == "" {
			logMessage("Current external IP: %s", ip)
		} else {
			logMessage("IP changed: %s -> %s", config.LastKnownIP, ip)
		}
		config.LastKnownIP = ip
	}
}

// Response from ipify API
type IPifyResponse struct {
	IP string `json:"ip"`
}

// fetchExternalIP makes an HTTP request to the specified endpoint and extracts the IP
// It will retry up to maxAttempts times if the request fails
func fetchExternalIP(endpoint string, attempt int, maxAttempts int) (string, error) {
	// Create HTTP client with a timeout
	client := http.Client{
		Timeout: 5 * time.Second,
	}
	
	// If we've already reached max attempts, fail immediately
	if attempt > maxAttempts {
		return "", fmt.Errorf("failed to make request after %d attempts", maxAttempts)
	}
	
	// Make request
	resp, err := client.Get(endpoint)
	if err != nil {
		// If we haven't reached max attempts yet, try again
		if attempt < maxAttempts {
			return fetchExternalIP(endpoint, attempt+1, maxAttempts)
		}
		return "", fmt.Errorf("failed to make request after %d attempts: %v", maxAttempts, err)
	}
	defer resp.Body.Close()
	
	// Check status code - only proceed with 2xx responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// If we haven't reached max attempts yet, try again
		if attempt < maxAttempts {
			return fetchExternalIP(endpoint, attempt+1, maxAttempts)
		}
		return "", fmt.Errorf("failed after %d attempts: HTTP status %d", maxAttempts, resp.StatusCode)
	}
	
	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}
	
	// Try to parse as JSON format (e.g., from ipify)
	var ipResponse IPifyResponse
	if err := json.Unmarshal(body, &ipResponse); err == nil && ipResponse.IP != "" {
		return ipResponse.IP, nil
	}
	
	// If JSON parsing fails, assume response is plain text
	return string(body), nil
}
