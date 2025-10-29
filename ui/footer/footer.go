package footer

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the footer's state
type Model struct {
	width        int
	mapShapePath string
	zoomLevel    float64
	lastPacket   string // --- NEW ---
}

// New creates a new footer model
func New(mapShapePath string) Model {
	return Model{
		width:        80, // Default
		mapShapePath: mapShapePath,
		zoomLevel:    1.0,
		lastPacket:   "---", // --- NEW ---
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// SetZoom allows the parent model to update the zoom level
func (m *Model) SetZoom(z float64) {
	m.zoomLevel = z
}

// --- NEW FUNCTION ---
// SetLastPacket allows the parent model to update the last packet
func (m *Model) SetLastPacket(call string) {
	m.lastPacket = call
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width // Just store the width
	}
	return m, nil
}

func (m Model) View() string {
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(0, 1)

	// --- UPDATED ---
	// Show Last Packet and Zoom
	footerLeft := footerStyle.Render(fmt.Sprintf(
		"PacketMap | Last: %-9s | Zoom: %.1fx", // %-9s pads the callsign
		m.lastPacket,
		m.zoomLevel,
	))

	footerHelp := "Pan: j/k/l/; | Zoom: K/L | Reset: r | Quit: q"

	// Use the component's width
	footerRight := footerStyle.Width(m.width - lipgloss.Width(footerLeft) - 1).
		Align(lipgloss.Right).
		Render(footerHelp)

	return lipgloss.JoinHorizontal(lipgloss.Left, footerLeft, footerRight)
}