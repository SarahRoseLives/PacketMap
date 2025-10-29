package mapview

import (
	"fmt"
	"log"
	"packetmap/config" // Import config
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
}

// loadMapData reads the shapefile and computes bounding box manually
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

// New creates a new map model
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

func (m Model) Init() tea.Cmd {
	return nil
}

// setCenterAndZoom sets the view to a specific center point and absolute zoom level
func (m *Model) setCenterAndZoom(lon, lat, zoomLevel float64) {
	// Calculate new dimensions based on original bounds and zoom level
	newWidth := (m.originalBounds.MaxX - m.originalBounds.MinX) / zoomLevel
	newHeight := (m.originalBounds.MaxY - m.originalBounds.MinY) / zoomLevel

	// Set new bounds centered on the given lon/lat
	m.viewBounds.MinX = lon - (newWidth / 2)
	m.viewBounds.MaxX = lon + (newWidth / 2)
	m.viewBounds.MinY = lat - (newHeight / 2)
	m.viewBounds.MaxY = lat + (newHeight / 2)
}

// zoomByFactor zooms the viewBounds in or out, centered on the current view
func (m *Model) zoomByFactor(factor float64) {
	// Get current center and dimensions
	centerX := (m.viewBounds.MinX + m.viewBounds.MaxX) / 2
	centerY := (m.viewBounds.MinY + m.viewBounds.MaxY) / 2
	width := m.viewBounds.MaxX - m.viewBounds.MinX
	height := m.viewBounds.MaxY - m.viewBounds.MinY

	// Calculate new dimensions
	newWidth := width * factor
	newHeight := height * factor

	// Don't zoom out further than the original map
	if newWidth > (m.originalBounds.MaxX-m.originalBounds.MinX) || newHeight > (m.originalBounds.MaxY-m.originalBounds.MinY) {
		m.viewBounds = m.originalBounds
		return
	}

	// Set new bounds centered on the same point
	m.viewBounds.MinX = centerX - (newWidth / 2)
	m.viewBounds.MaxX = centerX + (newWidth / 2)
	m.viewBounds.MinY = centerY - (newHeight / 2)
	m.viewBounds.MaxY = centerY + (newHeight / 2)
}

// pan moves the viewBounds
func (m *Model) pan(dx, dy float64) {
	width := m.viewBounds.MaxX - m.viewBounds.MinX
	height := m.viewBounds.MaxY - m.viewBounds.MinY

	// Calculate pan delta
	panX := width * dx
	panY := height * dy

	// Apply pan
	m.viewBounds.MinX += panX
	m.viewBounds.MaxX += panX
	m.viewBounds.MinY += panY
	m.viewBounds.MaxY += panY
}

// GetZoomLevel returns the current zoom factor
func (m Model) GetZoomLevel() float64 {
	// Avoid divide by zero
	if m.viewBounds.MaxX == m.viewBounds.MinX {
		return 1.0
	}
	return (m.originalBounds.MaxX - m.originalBounds.MinX) / (m.viewBounds.MaxX - m.viewBounds.MinX)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height // This is the height *given* by the parent

	case tea.KeyMsg:
		switch msg.String() {
		// --- Panning ---
		case "k", "up":
			m.pan(0, panFactor) // Pan Up (move map down, so increase Y)
		case "l", "down":
			m.pan(0, -panFactor) // Pan Down (move map up, so decrease Y)
		case "j", "left":
			m.pan(-panFactor, 0) // Pan Left (move map right, so decrease X)
		case ";", "right":
			m.pan(panFactor, 0) // Pan Right (move map left, so increase X)

		// --- Zooming ---
		case "K": // Zoom In (Shift+k)
			m.zoomByFactor(1 / zoomFactor) // Zoom in (divide by factor)
		case "L": // Zoom Out (Shift+l)
			m.zoomByFactor(zoomFactor) // Zoom out (multiply by factor)

		// --- Reset ---
		case "r":
			m.viewBounds = m.originalBounds
		}
	}

	return m, nil
}

// project converts lon/lat to terminal x/y coordinates
func (m *Model) project(lon, lat float64, viewWidth, viewHeight int) (int, int) {
	// Project based on the *current viewBounds*, not the original bounds

	// Avoid divide by zero if perfectly zoomed in
	if m.viewBounds.MaxX == m.viewBounds.MinX {
		m.viewBounds.MaxX += 1e-6
	}
	if m.viewBounds.MaxY == m.viewBounds.MinY {
		m.viewBounds.MaxY += 1e-6
	}

	// Normalize coordinates to [0, 1] based on the current view
	x := (lon - m.viewBounds.MinX) / (m.viewBounds.MaxX - m.viewBounds.MinX)
	y := (m.viewBounds.MaxY - lat) / (m.viewBounds.MaxY - m.viewBounds.MinY)

	// Scale to TUI dimensions
	tuiX := int(x * float64(viewWidth))
	tuiY := int(y * float64(viewHeight))
	return tuiX, tuiY
}

// renderMapViewport generates ASCII map
func (m Model) renderMapViewport(viewWidth, viewHeight int) string {
	if viewWidth <= 0 {
		viewWidth = 1 // Avoid zero-size grid
	}
	if viewHeight <= 0 {
		viewHeight = 1 // Avoid zero-size grid
	}

	grid := make([][]rune, viewHeight)
	for i := range grid {
		// --- THIS IS THE FIX ---
		// Your error message implies you had 'make([][]rune, viewWidth)'
		// It must be 'make([]rune, viewWidth)' with only one set of brackets.
		grid[i] = make([]rune, viewWidth)
		// ---
		for j := range grid[i] {
			grid[i][j] = ' '
		}
	}

	for _, polygon := range m.mapPolygons {
		polyBounds := polygon.BBox()
		if polyBounds.MaxX < m.viewBounds.MinX ||
			polyBounds.MinX > m.viewBounds.MaxX ||
			polyBounds.MaxY < m.viewBounds.MinY ||
			polyBounds.MinY > m.viewBounds.MaxY {
			continue // Skip this polygon, it's not in view
		}

		for _, point := range polygon.Points {
			x, y := m.project(point.X, point.Y, viewWidth, viewHeight)
			// Draw if projected point is within the TUI grid
			if x >= 0 && x < viewWidth && y >= 0 && y < viewHeight {
				grid[y][x] = '.'
			}
		}
	}

	// Plot the station's "house"
	if m.stationExists {
		x, y := m.project(m.stationLon, m.stationLat, viewWidth, viewHeight)
		// Draw if projected point is within the TUI grid
		if x >= 0 && x < viewWidth && y >= 0 && y < viewHeight {
			grid[y][x] = 'H' // 'H' for Home/House
		}
	}

	var b strings.Builder
	for _, row := range grid {
		b.WriteString(string(row))
		b.WriteRune('\n')
	}
	return b.String()
}

func (m Model) View() string {
	// Define the style inside the View, so it recalculates on resize
	mapStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(m.width - 2).   // Use component's width (minus borders)
		Height(m.height - 2) // Use component's height (minus borders)

	// Get the *content* area size, which is the style's size minus borders/padding
	hBorders := mapStyle.GetBorderLeftSize() + mapStyle.GetBorderRightSize()
	vBorders := mapStyle.GetBorderTopSize() + mapStyle.GetBorderBottomSize()
	hPadding := mapStyle.GetPaddingLeft() + mapStyle.GetPaddingRight()
	vPadding := mapStyle.GetPaddingTop() + mapStyle.GetPaddingBottom()

	// Note: GetWidth() and GetHeight() on a style are the *outer* dimensions
	mapViewWidth := mapStyle.GetWidth() - hBorders - hPadding
	mapViewHeight := mapStyle.GetHeight() - vBorders - vPadding

	if mapViewWidth <= 0 {
		mapViewWidth = 1
	}
	if mapViewHeight <= 0 {
		mapViewHeight = 1
	}

	mapContent := m.renderMapViewport(mapViewWidth, mapViewHeight)

	// Render the final styled block
	return mapStyle.Render(mapContent)
}