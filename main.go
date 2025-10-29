package main

import (
	"fmt" // Import fmt
	"log"
	"packetmap/config"
	"packetmap/device/aprsis" // --- NEW ---
	"packetmap/device/kiss"
	"packetmap/packet"
	"packetmap/ui/footer"
	mapview "packetmap/ui/map"
	"packetmap/ui/sidebar"
	"strings" // Import strings

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// mapShapePath is the path to your downloaded shapefile
const mapShapePath = "mapdata/ne_10m_admin_1_states_provinces.shp"

// --- NEW: Interface for device clients ---
// This allows main.go to handle either KISS or APRSIS clients
type PacketClient interface {
	Start(chan<- *packet.Packet)
	Close()
	// Add SetFilter later if needed: SetFilter(filter string) error
}

// --- Constants for Layout ---
const sidebarWidth = 20

// model holds the application's state
type model struct {
	width  int // Terminal width
	height int // Terminal height

	mapModel     mapview.Model // The map component
	footerModel  footer.Model  // The footer component
	sidebarModel sidebar.Model

	packetClient PacketClient // --- UPDATED: Use interface type ---
	packetChan   chan *packet.Packet

	err error // Store any errors
}

// initialModel creates the starting model
func initialModel(conf config.Config, client PacketClient, pChan chan *packet.Packet) model {
	// Create the map model, passing the entire config
	mapMod, err := mapview.New(mapShapePath, conf)
	if err != nil {
		return model{err: err} // Store the loading error
	}

	footerMod := footer.New(mapShapePath)
	sidebarMod := sidebar.New()

	// Set the initial zoom on the footer
	footerMod.SetZoom(mapMod.GetZoomLevel())

	return model{
		mapModel:     mapMod,
		footerModel:  footerMod,
		sidebarModel: sidebarMod,
		packetClient: client, // --- UPDATED ---
		packetChan:   pChan,
	}
}

// This is a tea.Cmd that blocks and waits for a packet on our channel
func (m model) listenForPackets() tea.Cmd {
	return func() tea.Msg {
		pkt := <-m.packetChan // Wait for a packet
		if pkt == nil {
			// Channel was closed
			// --- NEW: Send error message ---
			return fmt.Errorf("connection closed")
		}
		return pkt
	}
}

func (m model) Init() tea.Cmd {
	// Start the TNC client in a goroutine
	go m.packetClient.Start(m.packetChan) // --- UPDATED ---
	// Return the *first* listening command
	return m.listenForPackets()
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
		mapCmd     tea.Cmd
		footerCmd  tea.Cmd
		sidebarCmd tea.Cmd
		cmds       []tea.Cmd
	)

	switch msg := msg.(type) {
	case *packet.Packet:
		// Send packet to all three children
		m.mapModel, mapCmd = m.mapModel.Update(msg)
		m.footerModel.SetLastPacket(msg.Callsign)
		m.sidebarModel.AddPacket(msg.Callsign)

		cmds = append(cmds, mapCmd)
		// *** CRUCIAL: Re-queue the listener to wait for the *next* packet ***
		cmds = append(cmds, m.listenForPackets())

	// --- NEW: Handle connection closed error ---
	case error:
		m.err = msg // Store the error to display it
		log.Printf("Error received in Update: %v", msg) // Also log it
		return m, tea.Quit                           // Quit on connection error

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout Logic
		footerHeight := 1
		mainHeight := m.height - footerHeight
		mapWidth := m.width - sidebarWidth

		// Send resized messages
		sidebarMsg := tea.WindowSizeMsg{Width: sidebarWidth, Height: mainHeight}
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(sidebarMsg)

		mapMsg := tea.WindowSizeMsg{Width: mapWidth, Height: mainHeight}
		m.mapModel, mapCmd = m.mapModel.Update(mapMsg)

		footerMsg := tea.WindowSizeMsg{Width: m.width, Height: footerHeight}
		m.footerModel, footerCmd = m.footerModel.Update(footerMsg)

		cmds = append(cmds, sidebarCmd, mapCmd, footerCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		default:
			// Pass keys to the map model
			m.mapModel, mapCmd = m.mapModel.Update(msg)
			cmds = append(cmds, mapCmd)
			m.footerModel.SetZoom(m.mapModel.GetZoomLevel())
		}

	default:
		// Pass other messages
		m.mapModel, mapCmd = m.mapModel.Update(msg)
		m.footerModel, footerCmd = m.footerModel.Update(msg)
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(msg)
		cmds = append(cmds, mapCmd, footerCmd, sidebarCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	// Error View
	if m.err != nil {
		errorStyle := lipgloss.NewStyle().
			Width(m.width).
			Height(m.height).
			Border(lipgloss.DoubleBorder(), true).
			BorderForeground(lipgloss.Color("9")).
			Padding(1).
			Align(lipgloss.Center, lipgloss.Center)
		return errorStyle.Render(
			"Error:\n\n" + m.err.Error() +
				"\n\nPress any key to quit.",
		)
	}

	// Normal View
	sidebarView := m.sidebarModel.View()
	mapView := m.mapModel.View()
	footerView := m.footerModel.View()

	// Stack sidebar and map horizontally
	topStack := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarView,
		mapView,
	)

	// Join top stack and footer vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		topStack,
		footerView,
	)
}

func main() {
	// Load Config
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config.toml: %v", err)
	}

	// --- UPDATED: Connect based on config type ---
	var packetClient PacketClient // Use the interface
	var connectErr error

	switch strings.ToUpper(conf.Interface.Type) {
	case "KISS":
		packetClient, connectErr = kiss.Connect(conf.Interface)
	case "APRSIS":
		// APRSIS Connect needs the full config to get callsign
		packetClient, connectErr = aprsis.Connect(conf)
	default:
		connectErr = fmt.Errorf("unknown interface type in config: %s", conf.Interface.Type)
	}

	if connectErr != nil {
		log.Fatalf("Failed to connect to interface: %v", connectErr)
	}
	defer packetClient.Close() // Close whichever client we connected

	// Create packet channel
	packetChan := make(chan *packet.Packet)

	// Run Bubble Tea
	p := tea.NewProgram(initialModel(conf, packetClient, packetChan), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}