package kiss

import (
	"fmt"
	"net"
	"time"
)

// connectTCP dials a KISS TNC at the given address (e.g., "192.168.1.30:8001")
func connectTCP(address string) (net.Conn, error) {
	if address == "" {
		return nil, fmt.Errorf("no device address (ip:port) provided for KISS TCP")
	}

	// Dial with a timeout
	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to KISS TNC at %s: %w", address, err)
	}

	// We have a successful connection
	return conn, nil
}