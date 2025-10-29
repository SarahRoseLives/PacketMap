package main

import (
	"log"

	"packetmap/config"
	"packetmap/device/kiss"
	"packetmap/packet"
	"packetmap/ui/footer"
	mapview "packetmap/ui/map"
	"packetmap/ui/sidebar"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// mapShapePath is the path to your downloaded shapefile
const mapShapePath = "mapdata/ne_10m_admin_1_states_provinces.shp"

// --- REMOVED: type newPacketMsg *packet.Packet ---
// We will use *packet.Packet directly

// --- Constants for Layout ---
const sidebarWidth = 20

// model holds the application's state
type model struct {
	width  int // Terminal width
	height int // Terminal height

	mapModel     mapview.Model // The map component
	footerModel  footer.Model  // The footer component
	sidebarModel sidebar.Model

	kissClient *kiss.Client
	packetChan chan *packet.Packet

	err error // Store any errors
}

// initialModel creates the starting model
func initialModel(conf config.Config, client *kiss.Client, pChan chan *packet.Packet) model {
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
		kissClient:   client,
		packetChan:   pChan,
	}
}

// This is a tea.Cmd that blocks and waits for a packet on our channel
func (m model) listenForPackets() tea.Cmd {
	return func() tea.Msg {
		pkt := <-m.packetChan // Wait for a packet
		if pkt == nil {
			// Channel was closed
			return nil
		}
		// --- FIX: Return the packet directly, not the aliased type ---
		return pkt
	}
}

func (m model) Init() tea.Cmd {
	// Start the TNC client in a goroutine
	go m.kissClient.Start(m.packetChan)
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
	// --- FIX: Case now handles the *packet.Packet type directly ---
	case *packet.Packet:
		// Send packet to all three children
		m.mapModel, mapCmd = m.mapModel.Update(msg)
		m.footerModel.SetLastPacket(msg.Callsign)
		m.sidebarModel.AddPacket(msg.Callsign)

		cmds = append(cmds, mapCmd)
		// *** CRUCIAL: Re-queue the listener to wait for the *next* packet ***
		cmds = append(cmds, m.listenForPackets())

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// --- UPDATED LAYOUT LOGIC ---
		footerHeight := 1
		mainHeight := m.height - footerHeight // Height for map and sidebar
		mapWidth := m.width - sidebarWidth

		// Send resized messages to children
		sidebarMsg := tea.WindowSizeMsg{Width: sidebarWidth, Height: mainHeight}
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(sidebarMsg)

		mapMsg := tea.WindowSizeMsg{Width: mapWidth, Height: mainHeight}
		m.mapModel, mapCmd = m.mapModel.Update(mapMsg)

		// Footer gets FULL width
		footerMsg := tea.WindowSizeMsg{Width: m.width, Height: footerHeight}
		m.footerModel, footerCmd = m.footerModel.Update(footerMsg)

		cmds = append(cmds, sidebarCmd, mapCmd, footerCmd)

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
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(msg)
		cmds = append(cmds, mapCmd, footerCmd, sidebarCmd)
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
			"Error:\n\n" + m.err.Error() +
				"\n\nPress any key to quit.",
		)
	}

	// --- UPDATED: Horizontal Layout View ---

	// 1. Get the views from the children
	sidebarView := m.sidebarModel.View()
	mapView := m.mapModel.View()
	footerView := m.footerModel.View()

	// 2. Stack the sidebar and map horizontally
	topStack := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarView,
		mapView,
	)

	// 3. Join the top stack and the footer vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		topStack,
		footerView,
	)
}

func main() {
	// --- Load Config First ---
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config.toml: %v", err)
	}

	// --- Attempt to connect to TNC ---
	kissClient, err := kiss.Connect(conf.Interface)
	if err != nil {
		log.Fatalf("Failed to connect to interface: %v", err)
	}
	defer kissClient.Close()

	// --- Create the packet channel ---
	packetChan := make(chan *packet.Packet)

	// --- Pass client and channel to the initial model ---
	p := tea.NewProgram(initialModel(conf, kissClient, packetChan), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}