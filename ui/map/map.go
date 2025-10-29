package mapview

import (
	"fmt"
	"log"
	"packetmap/config"
	"packetmap/packet"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jonas-p/go-shp"
)

// Constants for Panning and Zooming
const (
	panFactor  = 0.1 // Pan 10% of the current view width/height
	zoomFactor = 1.2 // Zoom in/out by 20%
)

// --- REMOVED: type newPacketMsg *packet.Packet ---
// This type is now defined in main.go

// Model holds the map's state
type Model struct {
	width  int // Viewport width
	height int // Viewport height

	mapPolygons    []*shp.Polygon // Polygons from shapefile
	originalBounds shp.Box        // The bounds of the entire map, never changes
	viewBounds     shp.Box        // The bounds of the current viewport (pans and zooms)

	stationLon    float64 // Station Longitude (X)
	stationLat    float64 // Station Latitude (Y)
	stationExists bool    // Flag if station grid was loaded successfully

	plottedPackets []*packet.Packet
}

// loadMapData reads the shapefile... (NO CHANGES)
func loadMapData(path string) ([]*shp.Polygon, shp.Box, error) {
	shapeFile, err := shp.Open(path)
	if err != nil {
		return nil, shp.Box{}, fmt.Errorf("failed to open shapefile: %w", err)
	}
	defer shapeFile.Close()

	var polygons []*shp.Polygon
	bounds := shp.Box{
		MinX: 1e9,
		MinY: 1e9,
		MaxX: -1e9,
		MaxY: -1e9,
	}

	for shapeFile.Next() {
		_, shape := shapeFile.Shape()
		polygon, ok := shape.(*shp.Polygon)
		if !ok {
			continue
		}
		polygons = append(polygons, polygon)

		// Compute bounding box manually
		for _, p := range polygon.Points {
			if p.X < bounds.MinX {
				bounds.MinX = p.X
			}
			if p.X > bounds.MaxX {
				bounds.MaxX = p.X
			}
			if p.Y < bounds.MinY {
				bounds.MinY = p.Y
			}
			if p.Y > bounds.MaxY {
				bounds.MaxY = p.Y
			}
		}
	}

	if len(polygons) == 0 {
		return nil, shp.Box{}, fmt.Errorf("no polygons found in shapefile")
	}

	return polygons, bounds, nil
}

// New creates a new map model (NO CHANGES)
func New(mapShapePath string, conf config.Config) (Model, error) {
	polygons, bounds, err := loadMapData(mapShapePath)
	if err != nil {
		return Model{}, err
	}

	m := Model{
		mapPolygons:    polygons,
		originalBounds: bounds, // Store the original bounds
		viewBounds:     bounds, // The view starts fully zoomed out
		width:          80,     // Default width
		height:         23,     // Default height (24 - 1 for footer)
		stationExists:  false,  // Default
		plottedPackets: make([]*packet.Packet, 0),
	}

	// Try to parse the gridsquare from config
	stationGrid := conf.Station.GridSquare
	if stationGrid != "" {
		lon, lat, err := GridSquareToLatLon(stationGrid)
		if err != nil {
			log.Printf("Warning: Could not parse station gridsquare '%s': %v", stationGrid, err)
		} else {
			m.stationLon = lon
			m.stationLat = lat
			m.stationExists = true
		}
	}

	// Set initial view based on config
	if m.stationExists && conf.Map.DefaultZoom > 1.0 {
		m.setCenterAndZoom(m.stationLon, m.stationLat, conf.Map.DefaultZoom)
	}

	return m, nil
}

// Init, setCenterAndZoom, zoomByFactor, pan, GetZoomLevel...
// (NO CHANGES to these functions)

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) setCenterAndZoom(lon, lat, zoomLevel float64) {
	newWidth := (m.originalBounds.MaxX - m.originalBounds.MinX) / zoomLevel
	newHeight := (m.originalBounds.MaxY - m.originalBounds.MinY) / zoomLevel
	m.viewBounds.MinX = lon - (newWidth / 2)
	m.viewBounds.MaxX = lon + (newWidth / 2)
	m.viewBounds.MinY = lat - (newHeight / 2)
	m.viewBounds.MaxY = lat + (newHeight / 2)
}

func (m *Model) zoomByFactor(factor float64) {
	centerX := (m.viewBounds.MinX + m.viewBounds.MaxX) / 2
	centerY := (m.viewBounds.MinY + m.viewBounds.MaxY) / 2
	width := m.viewBounds.MaxX - m.viewBounds.MinX
	height := m.viewBounds.MaxY - m.viewBounds.MinY
	newWidth := width * factor
	newHeight := height * factor
	if newWidth > (m.originalBounds.MaxX-m.originalBounds.MinX) || newHeight > (m.originalBounds.MaxY-m.originalBounds.MinY) {
		m.viewBounds = m.originalBounds
		return
	}
	m.viewBounds.MinX = centerX - (newWidth / 2)
	m.viewBounds.MaxX = centerX + (newWidth / 2)
	m.viewBounds.MinY = centerY - (newHeight / 2)
	m.viewBounds.MaxY = centerY + (newHeight / 2)
}

func (m *Model) pan(dx, dy float64) {
	width := m.viewBounds.MaxX - m.viewBounds.MinX
	height := m.viewBounds.MaxY - m.viewBounds.MinY
	panX := width * dx
	panY := height * dy
	m.viewBounds.MinX += panX
	m.viewBounds.MaxX += panX
	m.viewBounds.MinY += panY
	m.viewBounds.MaxY += panY
}

func (m Model) GetZoomLevel() float64 {
	if m.viewBounds.MaxX == m.viewBounds.MinX {
		return 1.0
	}
	return (m.originalBounds.MaxX - m.originalBounds.MinX) / (m.viewBounds.MaxX - m.viewBounds.MinX)
}

// Update function (NO CHANGES)
// Note: It's okay that this case is `interface{}`. Go will match it.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case *packet.Packet: // Handle the packet message
		// We could just append, but let's replace if we already have it
		found := false
		for i, pkt := range m.plottedPackets {
			if pkt.Callsign == msg.Callsign {
				m.plottedPackets[i] = msg // Update position
				found = true
				break
			}
		}
		if !found {
			m.plottedPackets = append(m.plottedPackets, msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height // This is the height *given* by the parent

	case tea.KeyMsg:
		switch msg.String() {
		// Panning
		case "k", "up":
			m.pan(0, panFactor)
		case "l", "down":
			m.pan(0, -panFactor)
		case "j", "left":
			m.pan(-panFactor, 0)
		case ";", "right":
			m.pan(panFactor, 0)
		// Zooming
		case "K":
			m.zoomByFactor(1 / zoomFactor)
		case "L":
			m.zoomByFactor(zoomFactor)
		// Reset
		case "r":
			m.viewBounds = m.originalBounds
		}
	}

	return m, nil
}

// project converts lon/lat to terminal x/y coordinates (NO CHANGES)
func (m *Model) project(lon, lat float64, viewWidth, viewHeight int) (int, int) {
	if m.viewBounds.MaxX == m.viewBounds.MinX {
		m.viewBounds.MaxX += 1e-6
	}
	if m.viewBounds.MaxY == m.viewBounds.MinY {
		m.viewBounds.MaxY += 1e-6
	}
	x := (lon - m.viewBounds.MinX) / (m.viewBounds.MaxX - m.viewBounds.MinX)
	y := (m.viewBounds.MaxY - lat) / (m.viewBounds.MaxY - m.viewBounds.MinY)
	tuiX := int(x * float64(viewWidth))
	tuiY := int(y * float64(viewHeight))
	return tuiX, tuiY
}

// renderMapViewport (NO CHANGES)
func (m Model) renderMapViewport(viewWidth, viewHeight int) string {
	if viewWidth <= 0 {
		viewWidth = 1
	}
	if viewHeight <= 0 {
		viewHeight = 1
	}

	grid := make([][]rune, viewHeight)
	for i := range grid {
		grid[i] = make([]rune, viewWidth)
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	// 1. Draw the map
	for _, polygon := range m.mapPolygons {
		polyBounds := polygon.BBox()
		if polyBounds.MaxX < m.viewBounds.MinX ||
			polyBounds.MinX > m.viewBounds.MaxX ||
			polyBounds.MaxY < m.viewBounds.MinY ||
			polyBounds.MinY > m.viewBounds.MaxY {
			continue
		}

		for _, point := range polygon.Points {
			x, y := m.project(point.X, point.Y, viewWidth, viewHeight)
			if x >= 0 && x < viewWidth && y >= 0 && y < viewHeight {
				grid[y][x] = '.'
			}
		}
	}

	// 2. Plot the home station "house"
	if m.stationExists {
		x, y := m.project(m.stationLon, m.stationLat, viewWidth, viewHeight)
		if x >= 0 && x < viewWidth && y >= 0 && y < viewHeight {
			grid[y][x] = 'H' // 'H' for Home/House
		}
	}

	// 3. Draw the packets and callsigns (drawn last, so they're on top)
	for _, pkt := range m.plottedPackets {
		x, y := m.project(pkt.Lon, pkt.Lat, viewWidth, viewHeight)
		if x >= 0 && x < viewWidth && y >= 0 && y < viewHeight {
			grid[y][x] = '*' // Plot the packet position

			// Draw callsign UNDER the packet
			if y+1 < viewHeight { // Check if we have space below
				callRunes := []rune(pkt.Callsign)
				// Center the callsign under the '*'
				startOffset := x - (len(callRunes) / 2)

				for i := 0; i < len(callRunes); i++ {
					plotX := startOffset + i
					// Check horizontal bounds
					if plotX >= 0 && plotX < viewWidth {
						// Only write over empty space
						if grid[y+1][plotX] == ' ' {
							grid[y+1][plotX] = callRunes[i]
						}
					}
				}
			}
		}
	}

	var b strings.Builder
	for _, row := range grid {
		b.WriteString(string(row))
		b.WriteRune('\n')
	}
	return b.String()
}

// View function (NO CHANGES)
func (m Model) View() string {
	mapStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(m.width - 2).
		Height(m.height - 2)

	hBorders := mapStyle.GetBorderLeftSize() + mapStyle.GetBorderRightSize()
	vBorders := mapStyle.GetBorderTopSize() + mapStyle.GetBorderBottomSize()
	hPadding := mapStyle.GetPaddingLeft() + mapStyle.GetPaddingRight()
	vPadding := mapStyle.GetPaddingTop() + mapStyle.GetPaddingBottom()

	mapViewWidth := mapStyle.GetWidth() - hBorders - hPadding
	mapViewHeight := mapStyle.GetHeight() - vBorders - vPadding

	if mapViewWidth <= 0 {
		mapViewWidth = 1
	}
	if mapViewHeight <= 0 {
		mapViewHeight = 1
	}

	mapContent := m.renderMapViewport(mapViewWidth, mapViewHeight)

	return mapStyle.Render(mapContent)
}