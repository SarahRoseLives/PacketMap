package sidebar

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the sidebar's state
type Model struct {
	width   int
	height  int
	packets []string // A list of callsigns
}

// New creates a new sidebar model
func New() Model {
	return Model{
		width:   20, // Default
		height:  24, // Default
		packets: make([]string, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

// AddPacket adds a new callsign to the top of the list
func (m *Model) AddPacket(callsign string) {
	// Add to the top
	m.packets = append([]string{callsign}, m.packets...)

	// Trim the list if it's too long
	// We subtract 2 to leave room for the header and border
	maxPackets := m.height - 2
	if maxPackets < 1 {
		maxPackets = 1
	}
	if len(m.packets) > maxPackets {
		m.packets = m.packets[:maxPackets]
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	}
	return m, nil
}

func (m Model) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Width(m.width - 2).   // -2 for border
		Height(m.height - 2). // -2 for border
		Padding(0, 1)

	// Create the header
	header := lipgloss.NewStyle().
		Bold(true).
		Underline(true).
		Width(m.width - 2 - 2). // -2 border, -2 padding
		Render("Last Packets")

	// Create the list of packets
	// We fill the rest of the available height
	contentHeight := (m.height - 2) - 1 // -2 border, -1 header
	if contentHeight < 0 {
		contentHeight = 0
	}

	var b strings.Builder
	for i, call := range m.packets {
		if i >= contentHeight {
			break // Stop if we run out of room
		}
		b.WriteString(fmt.Sprintf("%.*s\n", m.width-2-2, call)) // Truncate callsign
	}

	// Join header and content
	content := lipgloss.JoinVertical(lipgloss.Left, header, b.String())

	return style.Render(content)
}