package packet

// --- NEW ---
// PacketType defines the type of APRS data.
type PacketType int

const (
	TypePosition PacketType = iota // A position report
	TypeMessage                    // A message
	TypeUnknown                    // Unknown or unparsed
)

// --- END NEW ---

// Packet holds the simplified APRS data we care about.
type Packet struct {
	Callsign string     // Source callsign (always present)
	Type     PacketType // --- NEW: Packet type ---

	// Fields for TypePosition
	Lat float64
	Lon float64

	// Fields for TypeMessage
	MsgTo   string // Recipient
	MsgBody string // Message content
	MsgID   string // Message ID
}