package header

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model holds the header's state
type Model struct {
	width int
}

// New creates a new header model
func New() Model {
	return Model{
		width: 80, // Default width, will be updated
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width // Just store the width
	}
	return m, nil
}

func (m Model) View() string {
	title := "PacketMap"

	// Style for the header
	style := lipgloss.NewStyle().
		Bold(true).
		Background(lipgloss.Color("63")). // Purple background (matches map border)
		Foreground(lipgloss.Color("255")). // White text
		Width(m.width).                   // Full terminal width
		Align(lipgloss.Center)             // Center the text

	return style.Render(title)
}