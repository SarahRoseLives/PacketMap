package aprs

import (
	"fmt"
	"regexp" // NEW
	"strconv"
	"strings" // NEW
)

// --- NEW: Regex ported from aprslib ---
// This regex is the key to fixing the separator bugs.
// 1: lat_deg (dd)
// 2: lat_min (mm.mm)
// 3: lat_dir (N/S)
// 4: symbol_table (the separator, e.g., \ / S)
// 5: lon_deg (ddd)
// 6: lon_min (mm.mm)
// 7: lon_dir (E/W)
// 8: symbol (the icon)
// 9: body (comment)
var normalPosRegex = regexp.MustCompile(
	`^(\d{2})([0-9 ]{2}\.[0-9 ]{2})([NnSs])` + // Lat
		`([\/\\0-9A-Z])` + // Symbol Table (Separator)
		`(\d{3})([0-9 ]{2}\.[0-9 ]{2})([EeWw])` + // Lon
		`([\x21-\x7e])` + // Symbol
		`(.*)$`, // Comment
)

// parseLat converts APRS latitude (DDMM.hhN) to decimal degrees
// --- UPDATED to handle ambiguity spaces ---
func parseLat(degStr, minStr, dirStr string) (float64, error) {
	// Handle ambiguity (replace spaces with '5' for centering)
	minStr = strings.ReplaceAll(minStr, " ", "5")

	deg, err := strconv.ParseFloat(degStr, 64)
	if err != nil {
		return 0, err
	}
	min, err := strconv.ParseFloat(minStr, 64)
	if err != nil {
		return 0, err
	}

	decDeg := deg + (min / 60.0)

	if dirStr == "S" || dirStr == "s" {
		decDeg = -decDeg
	} else if dirStr != "N" && dirStr != "n" {
		return 0, fmt.Errorf("invalid latitude hemisphere: %s", dirStr)
	}
	return decDeg, nil
}

// parseLon converts APRS longitude (DDDMM.hhW) to decimal degrees
// --- UPDATED to handle ambiguity spaces ---
func parseLon(degStr, minStr, dirStr string) (float64, error) {
	// Handle ambiguity
	minStr = strings.ReplaceAll(minStr, " ", "5")

	deg, err := strconv.ParseFloat(degStr, 64)
	if err != nil {
		return 0, err
	}
	min, err := strconv.ParseFloat(minStr, 64)
	if err != nil {
		return 0, err
	}

	decDeg := deg + (min / 60.0)

	if dirStr == "W" || dirStr == "w" {
		decDeg = -decDeg
	} else if dirStr != "E" && dirStr != "e" { // Fixed typo here
		return 0, fmt.Errorf("invalid longitude hemisphere: %s", dirStr)
	}
	return decDeg, nil
}

// --- RENAMED & REWRITTEN ---
// parseNormal handles uncompressed position reports ('!' and '/')
// It is the Go equivalent of aprslib.parsing.position.parse_normal
func parseNormal(payload string) (float64, float64, error) {
	// We expect the payload *with* the '!' or '/' prefix
	if len(payload) < 18 { // Min length for a valid ! or / packet
		return 0, 0, fmt.Errorf("packet too short")
	}

	// Body starts *after* the data type identifier
	body := payload[1:]
	dataType := payload[0]

	// Handle timestamp for '/' packets
	if dataType == '/' {
		// /HHMMSSz...
		if len(body) < 7 {
			return 0, 0, fmt.Errorf("timestamped packet too short")
		}
		// We don't parse the timestamp yet, just skip it
		body = body[7:]
	}

	// Use the regex to parse the position
	matches := normalPosRegex.FindStringSubmatch(body)
	if matches == nil {
		return 0, 0, fmt.Errorf("invalid uncompressed position format")
	}

	// matches[0] is the full string
	// matches[1] is lat_deg
	// matches[2] is lat_min
	// matches[3] is lat_dir
	// matches[4] is symbol_table (separator)
	// matches[5] is lon_deg
	// matches[6] is lon_min
	// matches[7] is lon_dir
	// matches[8] is symbol
	// matches[9] is comment

	lat, err := parseLat(matches[1], matches[2], matches[3])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse latitude: %w", err)
	}

	lon, err := parseLon(matches[5], matches[6], matches[7])
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse longitude: %w", err)
	}

	return lat, lon, nil
}

// parseUncompressedPosition is now just a wrapper for parseNormal
func parseUncompressedPosition(payload []byte) (float64, float64, error) {
	return parseNormal(string(payload))
}

// parseObjectPosition handles ';' data type (Object Report)
// Format: ;OBJECTNAME*HHMMSSzDDMM.hhN/DDDMM.hhW$...
// --- UPDATED to reuse parseNormal ---
func parseObjectPosition(payload []byte) (float64, float64, error) {
	sPayload := string(payload)
	dataType := sPayload[0]

	if dataType != ';' {
		return 0, 0, fmt.Errorf("not an object report")
	}

	// Min len: ; (1) + OBJNAME(9) + * (1) + TIME(7) + ...
	if len(sPayload) < 18 {
		return 0, 0, fmt.Errorf("object packet too short")
	}

	// Check for live '*' or dead '_' object marker
	if sPayload[10] != '*' && sPayload[10] != '_' {
		return 0, 0, fmt.Errorf("invalid object marker: %c", sPayload[10])
	}

	// The rest of the packet (from the timestamp on) is a normal position packet
	// Payload for parseNormal: /HHMMSSzDDMM.hhN/DDDMM.hhW$...
	// We make a new string starting with '/' and append the rest.
	posPayload := "/" + sPayload[11:]

	return parseNormal(posPayload)
}