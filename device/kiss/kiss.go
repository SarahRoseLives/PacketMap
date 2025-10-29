package kiss

import (
	"fmt"
	"io"
	"log" // Still used by Connect, which is fine
	"packetmap/aprs"
	"packetmap/config"
	"packetmap/packet"
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
		// Check if device is TCP or Serial
		if strings.Contains(conf.Device, ":") {
			// This is the existing TCP logic
			log.Printf("Attempting KISS TCP connection to: %s", conf.Device)
			tcpConn, err := connectTCP(conf.Device)
			if err != nil {
				return nil, err
			}
			log.Println("Successfully connected to KISS TNC via TCP")
			return &Client{conn: tcpConn}, nil

		} else {
			// This is the new Serial logic
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

// Start begins the packet-reading loop.
// It uses the Decoder to read frames and the aprs.Parse to parse them.
// Valid packets are sent down the provided channel.
// This function should be run as a goroutine.
func (c *Client) Start(packetChan chan<- *packet.Packet) {
	decoder := NewDecoder(c.conn)
	// log.Println("Starting KISS packet reader...") // REMOVED

	for {
		// ReadFrame blocks until a full frame is received
		frame, err := decoder.ReadFrame()
		if err != nil {
			// log.Printf("Error reading KISS frame: %v", err) // REMOVED
			// If the connection is closed, err will be io.EOF or similar
			close(packetChan) // Signal to the app that we're done
			return
		}

		// A valid KISS data frame has 0x00 as the first byte (port 0)
		if len(frame) < 1 || frame[0] != 0x00 {
			// log.Printf("Got non-data KISS frame (cmd %X), ignoring", frame[0])
			continue
		}

		// The rest of the frame is the AX.25 packet
		ax25Frame := frame[1:]

		// Try to parse it as APRS
		pkt, err := aprs.Parse(ax25Frame)
		if err != nil {
			// log.Printf("Failed to parse APRS: %v", err) // REMOVED
			// log.Printf("--- Failing Packet Start ---") // REMOVED
			// log.Printf("Payload (string): %s", string(ax25Frame)) // REMOVED
			// log.Printf("Payload (hex):    %x", ax25Frame) // REMOVED
			// log.Printf("--- Failing Packet End ---") // REMOVED
			continue
		}

		// Success! Send the packet to the main app
		// log.Printf("Got Packet: %s (%.3f, %.3f)", pkt.Callsign, pkt.Lat, pkt.Lon) // REMOVED
		packetChan <- pkt
	}
}

// Close disconnects the client
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}