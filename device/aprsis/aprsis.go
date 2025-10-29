package aprsis

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"packetmap/aprs"    // For parsing and passcode
	"packetmap/config"
	"packetmap/packet"
	mapview "packetmap/ui/map" // --- RE-ADDED: Import mapview for GridSquareToLatLon ---
	"strings"
	"time"
)

// APRS-IS server details
const (
	aprsisServer = "rotate.aprs.net:14580" // Using unfiltered port, but filter *command* might still work
	appName      = "PacketMap"
	appVersion   = "0.1"
	defaultRadiusKm = 200 // Default filter radius
)

// Client represents an active connection to an APRS-IS server
type Client struct {
	conn       net.Conn
	reader     *bufio.Reader
	callsign   string
	filter     string // --- RE-ADDED ---
	IsVerified bool
}

// Connect establishes a connection to an APRS-IS server
func Connect(conf config.Config) (*Client, error) {
	callsign := conf.Station.Callsign
	if callsign == "" {
		return nil, fmt.Errorf("callsign missing in config for APRS-IS")
	}
	passcode := conf.Interface.Passcode
	if passcode <= 0 {
		log.Printf("Warning: APRS-IS passcode not provided or invalid in config, connecting read-only.")
		passcode = -1
	} else {
		calculatedPasscode, err := aprs.CalculatePasscode(callsign)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate passcode: %w", err)
		}
		if passcode != calculatedPasscode {
			log.Printf("Warning: Provided passcode (%d) does not match calculated passcode (%d) for %s. Check config.toml. Connecting read-only.", passcode, calculatedPasscode, callsign)
			passcode = -1
		}
	}

	// --- RE-ADDED: Calculate filter based on gridsquare ---
	var filterStr string
	stationGrid := conf.Station.GridSquare
	if stationGrid != "" {
		lon, lat, err := mapview.GridSquareToLatLon(stationGrid)
		if err != nil {
			log.Printf("Warning: Could not parse station gridsquare '%s' for APRS-IS filter: %v. Using default filter.", stationGrid, err)
			// Fallback to a wide default if gridsquare is invalid
			filterStr = fmt.Sprintf("r/%.3f/%.3f/%d", 41.5, -81.0, defaultRadiusKm*2) // Centered roughly on Ohio
		} else {
			log.Printf("Setting APRS-IS filter based on gridsquare %s (Lat: %.3f, Lon: %.3f)", stationGrid, lat, lon)
			// TODO: Make radius configurable?
			filterStr = fmt.Sprintf("r/%.3f/%.3f/%d", lat, lon, defaultRadiusKm)
		}
	} else {
		log.Printf("Warning: Station gridsquare not found in config. Using default APRS-IS filter.")
		// Fallback to a wide default if no gridsquare is set
		filterStr = fmt.Sprintf("r/%.3f/%.3f/%d", 41.5, -81.0, defaultRadiusKm*2) // Centered roughly on Ohio
	}
	// --- End Re-added Filter Logic ---

	log.Printf("Attempting APRS-IS connection to %s", aprsisServer) // Log still mentions server:port
	conn, err := net.DialTimeout("tcp", aprsisServer, 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to APRS-IS server %s: %w", aprsisServer, err)
	}
	log.Printf("Connected to APRS-IS server: %s", conn.RemoteAddr())

	client := &Client{
		conn:     conn,
		reader:   bufio.NewReader(conn),
		callsign: callsign,
		filter:   filterStr, // --- RE-ADDED ---
	}

	// Perform login
	err = client.login(passcode)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("APRS-IS login failed: %w", err)
	}

	log.Println("APRS-IS Login successful")
	return client, nil
}

// login sends the login string and verifies the response
func (c *Client) login(passcode int) error {
	// --- RE-ADDED: Filter command in login string ---
	loginStr := fmt.Sprintf("user %s pass %d vers %s %s filter %s\r\n",
		c.callsign, passcode, appName, appVersion, c.filter)

	log.Printf("Sending login: user %s pass %s vers %s %s filter %s", // --- RE-ADDED: filter in log ---
		c.callsign, "****", appName, appVersion, c.filter)

	_, err := c.conn.Write([]byte(loginStr))
	if err != nil {
		return fmt.Errorf("failed to send login string: %w", err)
	}

	// Read lines until we get the verification line or an error
	// Set a timeout for the login response phase
	c.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer c.conn.SetReadDeadline(time.Time{}) // Clear deadline after login attempt

	for {
		lineBytes, err := c.reader.ReadBytes('\n')
		if err != nil {
			// Check for timeout specifically
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return fmt.Errorf("timeout waiting for login response from server")
			}
			if err == io.EOF {
				return fmt.Errorf("connection closed unexpectedly during login")
			}
			return fmt.Errorf("error reading login response: %w", err)
		}
		line := strings.TrimSpace(string(lineBytes))
		log.Printf("APRS-IS Server: %s", line)

		if strings.HasPrefix(line, "# logresp ") {
			// # logresp <callsign> verified|invalid ..., server <serverid>
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				if parts[2] == c.callsign {
					if strings.HasPrefix(parts[3], "verified") {
						c.IsVerified = (passcode != -1) // Verified only if we sent a real passcode
						return nil                      // Success!
					} else {
						// Even if login is "invalid" (e.g. bad passcode), server might keep connection open read-only.
						// Treat this as success for read-only purposes.
						log.Printf("APRS-IS login status: %s (continuing read-only)", parts[3])
						c.IsVerified = false
						return nil
					}
				} else {
					// Got a response for a different callsign? Unexpected.
					return fmt.Errorf("login response callsign mismatch: expected %s, got %s", c.callsign, parts[2])
				}
			}
		} else if strings.HasPrefix(line, "# Port ") || strings.HasPrefix(line, "# ") {
			continue // Ignore comments and port info
		} else {
			// If we receive actual APRS data before logresp, something is wrong or server is weird
			log.Printf("Received unexpected data before login confirmation: %s", line)
			// Let's assume login worked read-only if we didn't get an explicit failure
			c.IsVerified = false
			return nil
		}
	}
}

// Start begins the packet-reading loop for APRS-IS.
func (c *Client) Start(packetChan chan<- *packet.Packet) {
	log.Println("Starting APRS-IS packet reader...")

	for {
		c.conn.SetReadDeadline(time.Time{}) // Ensure no lingering deadline

		lineBytes, err := c.reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading APRS-IS stream: %v", err)
			} else {
				log.Println("APRS-IS connection closed.")
			}
			close(packetChan)
			return
		}

		line := strings.TrimSpace(string(lineBytes))

		// Ignore comments and empty lines
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Try to parse the line as an APRS packet
		pkt, err := aprs.Parse([]byte(line))
		if err != nil {
			// log.Printf("Failed to parse APRS-IS line: %v -- Line: %s", err, line) // Keep commented out
			continue
		}

		packetChan <- pkt
	}
}

// Close disconnects the client
func (c *Client) Close() {
	if c.conn != nil {
		log.Println("Closing APRS-IS connection.")
		c.conn.Close()
	}
}