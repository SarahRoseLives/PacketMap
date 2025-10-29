package kiss

import (
	"fmt"
	"io"
	"log"
	"packetmap/config"
	"strings"
)

// Client represents an active connection to a KISS TNC
type Client struct {
	conn io.ReadWriteCloser // The underlying connection (TCP, Serial, etc.)
}

// Connect establishes a connection to a TNC based on the interface config
func Connect(conf config.InterfaceConfig) (*Client, error) {
	log.Printf("Connecting to interface type: %s", conf.Type)

	switch strings.ToUpper(conf.Type) {
	case "KISS":
		// --- NEW: Check if device is TCP or Serial ---
		// We'll use a simple heuristic: if it contains a ':', it's ip:port (TCP).
		// Otherwise, it's a device path (Serial).
		if strings.Contains(conf.Device, ":") {
			// --- This is the existing TCP logic ---
			log.Printf("Attempting KISS TCP connection to: %s", conf.Device)
			tcpConn, err := connectTCP(conf.Device)
			if err != nil {
				return nil, err
			}
			log.Println("Successfully connected to KISS TNC via TCP")
			return &Client{conn: tcpConn}, nil

		} else {
			// --- This is the new Serial logic ---
			log.Printf("Attempting KISS Serial connection to: %s", conf.Device)
			serialConn, err := connectSerial(conf.Device)
			if err != nil {
				return nil, err
			}
			log.Println("Successfully connected to KISS TNC via Serial")
			return &Client{conn: serialConn}, nil
		}

	case "APRSIS":
		return nil, fmt.Errorf("APRSIS connection not yet implemented")

	default:
		return nil, fmt.Errorf("unknown interface type: %s", conf.Type)
	}
}

// Close disconnects the client
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}