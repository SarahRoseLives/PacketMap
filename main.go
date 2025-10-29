package main

import (
	"log"

	"packetmap/config"
	"packetmap/device/kiss" // --- NEW: Import kiss package ---
	"packetmap/ui/footer"
	mapview "packetmap/ui/map"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// mapShapePath is the path to your downloaded shapefile
const mapShapePath = "mapdata/ne_10m_admin_1_states_provinces.shp"

// model holds the application's state
type model struct {
	width  int // Terminal width
	height int // Terminal height

	mapModel    mapview.Model // The map component
	footerModel footer.Model  // The footer component

	err error // Store any errors
}

// initialModel creates the starting model
// It now accepts the loaded config
func initialModel(conf config.Config) model {
	// Create the map model, passing the entire config
	mapMod, err := mapview.New(mapShapePath, conf)
	if err != nil {
		return model{err: err} // Store the loading error
	}

	// Create the footer model
	footerMod := footer.New(mapShapePath)

	// Set the initial zoom on the footer
	footerMod.SetZoom(mapMod.GetZoomLevel())

	return model{
		mapModel:    mapMod,
		footerModel: footerMod,
	}
}

func (m model) Init() tea.Cmd {
	return nil // No initial commands
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// --- Global Error Handling ---
	if m.err != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit // Quit on any key if there's an error
		}
		return m, nil
	}

	var (
		mapCmd    tea.Cmd
		footerCmd tea.Cmd
		cmds      []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// --- Layout ---
		// Give the footer 1 line, and the map gets the rest
		footerHeight := 1
		mapHeight := m.height - footerHeight

		// Send resized messages to children
		mapMsg := tea.WindowSizeMsg{Width: m.width, Height: mapHeight}
		m.mapModel, mapCmd = m.mapModel.Update(mapMsg)

		footerMsg := tea.WindowSizeMsg{Width: m.width, Height: footerHeight}
		m.footerModel, footerCmd = m.footerModel.Update(footerMsg)

		cmds = append(cmds, mapCmd, footerCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		default:
			// Pass all other keys to the map model
			m.mapModel, mapCmd = m.mapModel.Update(msg)
			cmds = append(cmds, mapCmd)

			// Sync footer zoom level after map update
			m.footerModel.SetZoom(m.mapModel.GetZoomLevel())
		}

	default:
		// Pass any other messages to children (e.g., mouse)
		m.mapModel, mapCmd = m.mapModel.Update(msg)
		m.footerModel, footerCmd = m.footerModel.Update(msg)
		cmds = append(cmds, mapCmd, footerCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// --- Error View ---
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Border(lipgloss.DoubleBorder(), true).
			BorderForeground(lipgloss.Color("9")).
			Padding(1).
			Align(lipgloss.Center, lipgloss.Center)
		return errorStyle.Render(
			"Error:\n\n" + m.err.Error() + // Simplified error view
				"\n\nPress any key to quit.",
		)
	}

	// --- Normal View ---
	mapView := m.mapModel.View()
	footerView := m.footerModel.View()

	// Stack them vertically
	return lipgloss.JoinVertical(lipgloss.Left, mapView, footerView)
}

func main() {
	// --- Load Config First ---
	conf, err := config.LoadConfig()
	if err != nil {
		// If config fails, we must exit before starting Bubble Tea
		log.Fatalf("Failed to load config.toml: %v", err)
	}

	// --- NEW: Attempt to connect to TNC ---
	// We pass the Interface part of the config
	kissClient, err := kiss.Connect(conf.Interface)
	if err != nil {
		// If connection fails, we must exit
		log.Fatalf("Failed to connect to interface: %v", err)
	}
	// On exit, ensure we close the connection
	defer kissClient.Close()

	// --- Pass the loaded config to the initial model ---
	p := tea.NewProgram(initialModel(conf), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}