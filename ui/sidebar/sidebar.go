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
	// m.height is the total component height (mainHeight from main.go)
	// -2 for the top/bottom borders
	// -1 for the "Last Packets" header
	maxPackets := m.height - 3
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

		// --- NEW: Re-trim packet list on resize ---
		// This handles the case where the window gets smaller
		maxPackets := m.height - 3
		if maxPackets < 1 {
			maxPackets = 1
		}
		if len(m.packets) > maxPackets {
			m.packets = m.packets[:maxPackets]
		}
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

	// --- THIS IS THE FIX ---
	// Build the content string manually to respect the height

	var b strings.Builder
	b.WriteString(header) // Add header (1 line)

	// Calculate how many lines are left for packets
	// (Inner Height) - (Header Height)
	contentHeight := (m.height - 2) - 1
	if contentHeight < 0 {
		contentHeight = 0
	}

	if contentHeight > 0 { // Only add packets if there is space
		b.WriteRune('\n') // Add newline after header
		for i, call := range m.packets {
			if i >= contentHeight {
				break // Stop if we run out of room
			}
			// Truncate callsign to fit width
			line := fmt.Sprintf("%.*s", m.width-2-2, call)
			b.WriteString(line)
			if i < len(m.packets)-1 && i < contentHeight-1 { // Don't add newline to the very last item
				b.WriteRune('\n')
			}
		}
	}

	// Now, `b.String()` is a single string that has *at most*
	// `m.height - 2` lines.
	// We render this string *inside* the box, and the box
	// will not expand vertically.
	return style.Render(b.String())
}