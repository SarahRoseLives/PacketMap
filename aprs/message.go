package aprs

import (
	"fmt"
	"strings"
)

// parseMessage parses a message packet (data type ':').
// Format: :ADDRESSEE :message body{id
func parseMessage(payload []byte) (to, body, id string, err error) {
	sPayload := string(payload[1:]) // Skip data type ':'

	if len(sPayload) < 11 { // : (9 chars) + : (1 char) + m (1 char)
		return "", "", "", fmt.Errorf("message packet too short")
	}

	// 1. Find the addressee (must be 9 chars, padded with spaces)
	to = strings.TrimSpace(sPayload[0:9])
	if to == "" {
		return "", "", "", fmt.Errorf("message recipient is blank")
	}

	// 2. Check for the second ':'
	if sPayload[9] != ':' {
		return "", "", "", fmt.Errorf("missing message body separator ':'")
	}

	// 3. Find the message body and optional {id
	bodyPart := sPayload[10:]

	idIndex := strings.LastIndex(bodyPart, "{")
	if idIndex != -1 && idIndex > 0 { // Check > 0 to ensure it's not the first char
		// Found a potential message ID
		body = strings.TrimSpace(bodyPart[:idIndex])
		id = strings.TrimSpace(bodyPart[idIndex+1:])
	} else {
		// No message ID found
		body = strings.TrimSpace(bodyPart)
		id = ""
	}

	if body == "" {
		return "", "", "", fmt.Errorf("message body is blank")
	}

	return to, body, id, nil
}