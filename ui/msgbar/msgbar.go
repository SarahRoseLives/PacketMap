package msgbar

import (
	"fmt"
	"packetmap/packet"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	barHeight = 7// Total height of the component (including border)
)

// Model holds the message bar's state
type Model struct {
	width    int
	height   int
	messages []string // Stores formatted message strings
}

// New creates a new message bar model
func New() Model {
	return Model{
		width:    80,
		height:   barHeight,
		messages: make([]string, 0),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		// Height is fixed, but we store it for consistency
		m.height = barHeight

	case *packet.Packet:
		// We only get message packets, as filtered by main.go
		if msg.Type != packet.TypeMessage {
			return m, nil // Should not happen, but good to check
		}

		// Format the message
		// Example: N0CALL>KD2YCB: Hello world!
		line := fmt.Sprintf("%s>%s: %s", msg.Callsign, msg.MsgTo, msg.MsgBody)

		// Add to the top
		m.messages = append([]string{line}, m.messages...)

		// Trim the list if it's too long
		// barHeight - 2 (for borders)
		maxMessages := barHeight - 2
		if maxMessages < 1 {
			maxMessages = 1
		}
		if len(m.messages) > maxMessages {
			m.messages = m.messages[:maxMessages]
		}
	}
	return m, nil
}

func (m Model) View() string {
	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")). // Purple
		Width(m.width - 2).   // -2 for border
		Height(m.height - 2). // -2 for border
		Padding(0, 1)

	// Build the content
	var b strings.Builder

	// Available width for text
	contentWidth := m.width - 2 - 2 // -border, -padding
	if contentWidth < 0 {
		contentWidth = 0
	}

	// Get the messages that fit
	numMessages := m.height - 2
	if numMessages < 0 {
		numMessages = 0
	}

	for i := 0; i < numMessages; i++ {
		if i < len(m.messages) {
			// Get message from bottom up to show oldest first in bar
			// This shows messages in the order they arrived
			msg := m.messages[len(m.messages)-1-i]

			// Truncate
			if len(msg) > contentWidth {
				msg = msg[:contentWidth]
			}
			b.WriteString(msg)
		}
		if i < numMessages-1 {
			b.WriteRune('\n') // Add newline unless it's the last line
		}
	}

	return style.Render(b.String())
}