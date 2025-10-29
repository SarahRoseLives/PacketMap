package aprs

import (
	"bytes"
	"fmt"
	"packetmap/packet"
)

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
	var lat, lon float64

	switch dataType {
	// --- THIS IS THE FIX ---
	// Added '=' to the list of uncompressed position types
	case '!', '/', '=':
		// Uncompressed position report.
		lat, lon, err = parseUncompressedPosition(payload)
		if err != nil {
			return nil, fmt.Errorf("uncompressed position parse failed: %w", err)
		}

	case ';':
		// Object position report.
		lat, lon, err = parseObjectPosition(payload)
		if err != nil {
			// If it's not a valid object, it might be a malformed packet
			// with a '!' in it. Fall through to the default logic.
		} else {
			// Success! Break out.
			break
		}
		fallthrough // Fall through to default

	default:
		// Check if a '!' is in the body, as per python parser
		// This handles packets that don't have a valid data type prefix
		idx := bytes.IndexByte(payload, '!')
		if idx > 0 && idx < 40 { // aprslib's check
			// Found one. Treat payload as starting from here.
			payload = payload[idx:]
			lat, lon, err = parseUncompressedPosition(payload)
			if err != nil {
				return nil, fmt.Errorf("uncompressed position parse failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("unsupported APRS data type: %c", dataType)
		}
	}

	// 3. We successfully parsed a packet!
	return &packet.Packet{
		Callsign: callsign,
		Lat:      lat,
		Lon:      lon,
	}, nil
}