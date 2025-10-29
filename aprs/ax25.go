package aprs

import (
	"fmt"
	"strings"
)

// The AX.25 header contains addresses (callsigns)
// We need to find the Source address and the start of the APRS data.
// APRS data is in an "UI" (Unnumbered Information) frame.
// AX.25 Header:
// [Dest Addr: 7 bytes]
// [Src Addr: 7 bytes]
// [Path Addr 1: 7 bytes]...
// [Path Addr N: 7 bytes]
// [Control Field: 1 byte (0x03 for UI)]
// [PID Field: 1 byte (0xF0 for no layer 3)]
// [APRS Data starts here]

const (
	controlUI byte = 0x03
	pidNoLayer3 byte = 0xF0
)

// parseAddress decodes a 7-byte AX.25 address field.
func parseAddress(addr []byte) (string, byte) {
	if len(addr) != 7 {
		return "", 0
	}

	var callsign strings.Builder
	for i := 0; i < 6; i++ {
		char := addr[i] >> 1 // Shift right to get the ASCII value
		if char == ' ' || char == 0 {
			break
		}
		callsign.WriteByte(char)
	}

	// SSID is in the 7th byte
	ssidByte := addr[6]
	ssid := (ssidByte >> 1) & 0x0F

	if ssid > 0 {
		return fmt.Sprintf("%s-%d", callsign.String(), ssid), ssidByte
	}
	return callsign.String(), ssidByte
}

// findPayload searches the frame for the APRS payload (data after 0x03 0xF0)
// It returns the Source Callsign and the APRS payload itself.
func findPayload(frame []byte) (string, []byte, error) {
	if len(frame) < 16 { // Min size: Dest(7) + Src(7) + Ctrl(1) + PID(1)
		return "", nil, fmt.Errorf("frame too short")
	}

	// Skip Destination address (7 bytes)
	// Read Source address (next 7 bytes)
	srcCall, _ := parseAddress(frame[7:14])
	if srcCall == "" {
		return "", nil, fmt.Errorf("invalid source callsign")
	}

	// The end of the address path is marked by the last byte having its LSB set
	// We'll scan forward from the source address to find it.
	addrEndIndex := 14 // Start scanning after the source address
	for addrEndIndex < len(frame) {
		if (frame[addrEndIndex-1] & 0x01) == 0x01 {
			break // Found the last address field
		}
		addrEndIndex += 7 // Move to the next address
	}

	if addrEndIndex+2 > len(frame) {
		return "", nil, fmt.Errorf("could not find end of address path or control fields")
	}

	// The next two bytes *should* be Control (0x03) and PID (0xF0)
	controlField := frame[addrEndIndex]
	pidField := frame[addrEndIndex+1]

	if controlField != controlUI {
		return "", nil, fmt.Errorf("not a UI frame (control: 0x%02X)", controlField)
	}

	if pidField != pidNoLayer3 {
		return "", nil, fmt.Errorf("not a 'no layer 3' frame (PID: 0x%02X)", pidField)
	}

	// If we're here, the rest is the APRS payload
	payload := frame[addrEndIndex+2:]

	// --- THIS IS THE FIX ---
	// We remove the TNC2-style stripping logic.
	// It was too aggressive and was cutting off valid packets
	// that contained a ':' in their comment field.
	/*
		if colonIndex := bytes.IndexByte(payload, ':'); colonIndex != -1 {
			payload = payload[colonIndex+1:]
		}
	*/
	// --- End Fix ---

	return srcCall, payload, nil
}