package aprs

import (
	"fmt"
	"strings"
)

// CalculatePasscode generates the APRS-IS passcode for a given callsign.
// It's a port of the common passcode algorithm.
func CalculatePasscode(callsign string) (int, error) {
	// Use uppercase callsign without SSID
	call := strings.ToUpper(strings.Split(callsign, "-")[0])

	if len(call) > 6 || len(call) < 1 {
		return 0, fmt.Errorf("invalid callsign format for passcode: %s", callsign)
	}

	hash := 0x73e2 // Initialize with seed
	var flag bool  // Alternates between true/false

	for _, char := range call {
		// XOR with character shifted left by 8 if flag is false
		shift := 0
		if !flag {
			shift = 8
		}
		hash ^= int(char) << shift
		flag = !flag // Flip flag
	}

	// Mask with 0x7fff and return
	return hash & 0x7fff, nil
}