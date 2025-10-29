package aprs

import (
	"bytes"
	"fmt"
	"strings"
)

// AX.25 constants (less relevant for APRS-IS text format, but kept for context)
const (
	controlUI   byte = 0x03
	pidNoLayer3 byte = 0xF0
)

// parseAddress decodes a 7-byte AX.25 address field OR extracts callsign-SSID from text.
// For text, it assumes the input is just the callsign string.
func parseAddressText(callStr string) (string, error) {
	// Simple validation for APRS-IS text callsigns
	if len(callStr) == 0 {
		return "", fmt.Errorf("empty callsign string")
	}
	// Basic check, can be expanded if needed
	if !strings.ContainsAny(callStr, "->") { // Check it's not part of the path
		return callStr, nil
	}
	return "", fmt.Errorf("invalid characters in callsign string")

}

// findPayload searches the frame for the APRS payload.
// MODIFIED: It now handles both raw AX.25 bytes and APRS-IS text lines.
func findPayload(frame []byte) (string, []byte, error) {
	// Attempt to find standard APRS-IS text format first: CALL>DEST,PATH:payload
	frameStr := string(frame) // Work with strings for text parsing
	separatorIndex := strings.Index(frameStr, ":")
	if separatorIndex == -1 {
		// No colon found, maybe it's a raw AX.25 frame? Or malformed?
		// Fallback to trying AX.25 logic (less likely for APRS-IS)
		return findPayloadAX25(frame)
	}

	headerPart := frameStr[:separatorIndex]
	payload := frame[separatorIndex+1:]

	// Extract source callsign (part before '>')
	callEndIndex := strings.Index(headerPart, ">")
	if callEndIndex == -1 {
		return "", nil, fmt.Errorf("no source callsign separator '>' found in header: %s", headerPart)
	}
	srcCallStr := headerPart[:callEndIndex]

	// Basic validation of source call
	// We don't need full AX.25 validation here
	if len(srcCallStr) == 0 || len(srcCallStr) > 9 { // APRS callsigns can be up to 9 chars
		return "", nil, fmt.Errorf("invalid source callsign format: %s", srcCallStr)
	}

	return srcCallStr, []byte(payload), nil
}

// findPayloadAX25 handles the original logic for raw AX.25 frames (from KISS).
func findPayloadAX25(frame []byte) (string, []byte, error) {
	if len(frame) < 16 { // Min size: Dest(7) + Src(7) + Ctrl(1) + PID(1)
		return "", nil, fmt.Errorf("frame too short for AX.25")
	}

	// AX.25 Address parsing logic (requires byte manipulation)
	srcCall, _, err := parseAddressBytes(frame[7:14])
	if err != nil {
		return "", nil, fmt.Errorf("invalid AX.25 source address: %w", err)
	}

	// Find end of address path (LSB check)
	addrEndIndex := 14
	for addrEndIndex < len(frame) {
		if (frame[addrEndIndex-1] & 0x01) == 0x01 {
			break
		}
		// Sanity check: prevent infinite loop if LSB is never set
		if addrEndIndex+7 > len(frame)+14 { // Allow some room but prevent going way too far
			return "", nil, fmt.Errorf("could not find end of AX.25 address path (LSB never set?)")
		}
		addrEndIndex += 7
	}


	if addrEndIndex+2 > len(frame) {
		return "", nil, fmt.Errorf("could not find AX.25 control/PID fields after address path")
	}

	controlField := frame[addrEndIndex]
	pidField := frame[addrEndIndex+1]

	if controlField != controlUI {
		return "", nil, fmt.Errorf("not a UI frame (control: 0x%02X)", controlField)
	}

	if pidField != pidNoLayer3 {
		// Be lenient with PID for KISS TNCs that might omit it or use others
		// return "", nil, fmt.Errorf("not a 'no layer 3' frame (PID: 0x%02X)", pidField)
	}

	payload := frame[addrEndIndex+2:]

	// Raw AX.25 might still have TNC2 prefix sometimes from digipeaters, keep the check
	if colonIndex := bytes.IndexByte(payload, ':'); colonIndex != -1 {
		// Only strip if it looks like CALL>PATH: prefix
		if bytes.IndexByte(payload[:colonIndex], '>') != -1 {
			payload = payload[colonIndex+1:]
		}
	}


	return srcCall, payload, nil
}

// parseAddressBytes decodes a 7-byte AX.25 address field.
// Returns callsign string, SSID byte, and error.
func parseAddressBytes(addr []byte) (string, byte, error) {
	if len(addr) != 7 {
		return "", 0, fmt.Errorf("address length is not 7 bytes")
	}

	var callsign strings.Builder
	for i := 0; i < 6; i++ {
		char := addr[i] >> 1 // Shift right to get ASCII value
		// Allow spaces but break on null or invalid chars
		if char < ' ' || char > '~' {
			if char == 0 { // Null termination is fine if callsign is shorter
				break
			}
			// Skip invalid characters within the 6-byte callsign field if needed,
			// though strictly they shouldn't be there.
			// Consider returning error for strict AX.25 compliance:
			// return "", 0, fmt.Errorf("invalid character in callsign bytes: 0x%02X", addr[i])
			continue // Be lenient for now
		}
		if char != ' ' { // Don't append padding spaces
		    callsign.WriteByte(char)
		}
	}

    callStr := strings.TrimSpace(callsign.String())
    if len(callStr) == 0 {
        return "", 0, fmt.Errorf("decoded callsign is empty")
    }

	ssidByte := addr[6]
	ssid := (ssidByte >> 1) & 0x0F // Extract SSID bits

	if ssid > 0 {
		return fmt.Sprintf("%s-%d", callStr, ssid), ssidByte, nil
	}
	return callStr, ssidByte, nil
}