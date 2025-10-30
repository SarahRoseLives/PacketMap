package aprs

import (
	"bytes"
	"fmt"
	"packetmap/packet"
	"strings" // --- ADDED IMPORT ---
)

// --- NEW HELPER FUNCTION ---
var telemetryKeywords = []string{
	"PARM",
	"UNIT",
	"EQNS",
	"BITS",
	// We can add more known telemetry prefixes here
}

// isTelemetry checks if a message is likely an automated telemetry report
// based on keywords or being self-addressed.
func isTelemetry(from, to, body string) bool {
	// Filter 1: Check if it's self-addressed (common for telemetry)
	if from == to {
		return true
	}

	// Filter 2: Check for common telemetry keywords at the start of the body
	for _, kw := range telemetryKeywords {
		if strings.HasPrefix(body, kw) {
			return true
		}
	}

	// --- THIS IS THE NEW LINE ---
	// Filter 3: Check for NWS (National Weather Service) in the sender's callsign
	if strings.Contains(from, "NWS") {
		return true
	}
	// --- END NEW LINE ---

	return false
}

// --- END NEW HELPER FUNCTION ---

// Parse takes a raw AX.25 frame (payload from KISS) and returns our
// internal Packet type if it's a valid position report.
func Parse(rawFrame []byte) (*packet.Packet, error) {

	// 1. Parse the AX.25 header to get the callsign and APRS payload
	callsign, payload, err := findPayload(rawFrame)
	if err != nil {
		return nil, fmt.Errorf("AX.25 parse failed: %w", err)
	}

	if len(payload) == 0 {
		return nil, fmt.Errorf("empty APRS payload")
	}

	// 2. Check the APRS Data Type Identifier (first byte of payload)
	dataType := payload[0]

	// --- MODIFIED: Create empty packet first ---
	pkt := &packet.Packet{
		Callsign: callsign,
		Type:     packet.TypeUnknown, // Default to unknown
	}

	switch dataType {
	// --- THIS IS THE FIX ---
	// Added '=' to the list of uncompressed position types
	case '!', '/', '=':
		// Uncompressed position report.
		lat, lon, err := parseUncompressedPosition(payload)
		if err != nil {
			return nil, fmt.Errorf("uncompressed position parse failed: %w", err)
		}
		// --- MODIFIED: Populate packet ---
		pkt.Type = packet.TypePosition
		pkt.Lat = lat
		pkt.Lon = lon

	case ';':
		// Object position report.
		lat, lon, err := parseObjectPosition(payload)
		if err != nil {
			// If it's not a valid object, it might be a malformed packet
			// with a '!' in it. Fall through to the default logic.
		} else {
			// Success! Break out.
			// --- MODIFIED: Populate packet ---
			pkt.Type = packet.TypePosition
			pkt.Lat = lat
			pkt.Lon = lon
			break
		}
		fallthrough // Fall through to default

	// --- NEW: Handle Message Packets ---
	case ':':
		to, body, id, err := parseMessage(payload)
		if err != nil {
			return nil, fmt.Errorf("message parse failed: %w", err)
		}

		// --- NEW FILTERING LOGIC ---
		if isTelemetry(callsign, to, body) {
			// This is telemetry, not a user message.
			// We'll return an error, which causes the packet
			// to be silently ignored by the device loop.
			return nil, fmt.Errorf("ignoring telemetry/NWS packet: %s", body)
		}
		// --- END NEW FILTERING LOGIC ---

		pkt.Type = packet.TypeMessage
		pkt.MsgTo = to
		pkt.MsgBody = body
		pkt.MsgID = id
	// --- END NEW ---

	default:
		// Check if a '!' is in the body, as per python parser
		// This handles packets that don't have a valid data type prefix
		idx := bytes.IndexByte(payload, '!')
		if idx > 0 && idx < 40 { // aprslib's check
			// Found one. Treat payload as starting from here.
			payload = payload[idx:]
			lat, lon, err := parseUncompressedPosition(payload)
			if err != nil {
				return nil, fmt.Errorf("uncompressed position parse failed: %w", err)
			}
			// --- MODIFIED: Populate packet ---
			pkt.Type = packet.TypePosition
			pkt.Lat = lat
			pkt.Lon = lon
		} else {
			return nil, fmt.Errorf("unsupported APRS data type: %c", dataType)
		}
	}

	// 3. We successfully parsed a packet!
	if pkt.Type == packet.TypeUnknown {
		// This should be unreachable if the switch is correct
		return nil, fmt.Errorf("packet parsed but type is still unknown")
	}


	return pkt, nil
}