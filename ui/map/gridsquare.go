package mapview

import (
	"fmt"
	"strings"
)

// GridSquareToLatLon converts a Maidenhead gridsquare (like "EN91" or "EN91kl")
// to the longitude (X) and latitude (Y) of its center.
func GridSquareToLatLon(grid string) (float64, float64, error) {
	grid = strings.ToUpper(grid)
	if len(grid) < 4 {
		return 0, 0, fmt.Errorf("gridsquare too short: %s", grid)
	}

	// Field (e.g., "EN")
	// 'A' = -180, 'R' = 160
	lon := (float64(grid[0]-'A') * 20.0) - 180.0
	// 'A' = -90, 'R' = 80
	lat := (float64(grid[1]-'A') * 10.0) - 90.0

	// Square (e.g., "91")
	// '0' = 0, '9' = 18
	lon += (float64(grid[2]-'0') * 2.0)
	// '0' = 0, '9' = 9
	lat += (float64(grid[3]-'0') * 1.0)

	// Start with center of 4-char grid (1° lon, 0.5° lat)
	lon += 1.0
	lat += 0.5

	// Subsquare (e.g., "KL")
	if len(grid) >= 6 {
		// Remove 4-char center offset
		lon -= 1.0
		lat -= 0.5

		// Add subsquare offset
		// 'A' = 0, 'X' = 23 * (2.0/24.0)
		lon += (float64(grid[4]-'A') * (2.0 / 24.0)) // 5' resolution
		// 'A' = 0, 'X' = 23 * (1.0/24.0)
		lat += (float64(grid[5]-'A') * (1.0 / 24.0)) // 2.5' resolution

		// Add 6-char center offset (2.5' lon, 1.25' lat)
		lon += (1.0 / 24.0)
		lat += (0.5 / 24.0)
	}

	// Validate ranges
	if lon < -180.0 || lon > 180.0 || lat < -90.0 || lat > 90.0 {
		return 0, 0, fmt.Errorf("invalid gridsquare calculation for %s", grid)
	}

	return lon, lat, nil
}