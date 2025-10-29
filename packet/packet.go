package packet

// Packet holds the simplified APRS data we care about.
type Packet struct {
	Callsign string
	Lat      float64
	Lon      float64
	// We can add more fields later (comment, symbol, etc.)
}