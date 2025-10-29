package kiss

import (
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"
)

// connectSerial opens a connection to a serial KISS TNC
func connectSerial(devicePath string) (io.ReadWriteCloser, error) {
	if devicePath == "" {
		return nil, fmt.Errorf("no device path (e.g., /dev/ttyUSB0 or COM3) provided for KISS serial")
	}

	// TODO: The baud rate should be configurable in config.toml
	// For now, we default to 9600, a common rate for TNCs.
	mode := &serial.Mode{
		BaudRate: 9600,
	}

	port, err := serial.Open(devicePath, mode)
	if err != nil {
		return nil, fmt.Errorf("failed to open serial port %s: %w", devicePath, err)
	}

	// Set a read timeout so Read() doesn't block forever
	// This is critical for serial I/O.
	if err := port.SetReadTimeout(1 * time.Second); err != nil {
		port.Close() // Close the port if we can't set the timeout
		return nil, fmt.Errorf("failed to set read timeout: %w", err)
	}

	return port, nil
}