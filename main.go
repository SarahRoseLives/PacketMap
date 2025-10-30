package main

import (
	"fmt"
	"log"
	"os/exec" // --- ADDED ---
	"packetmap/config"
	"packetmap/device/aprsis"
	"packetmap/device/kiss"
	"packetmap/packet"
	"packetmap/ui/footer"
	"packetmap/ui/header"
	mapview "packetmap/ui/map"
	"packetmap/ui/msgbar"
	"packetmap/ui/sidebar"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// mapShapePath is the path to your downloaded shapefile
const mapShapePath = "mapdata/ne_10m_admin_1_states_provinces.shp"

// PacketClient defines the interface for TNC/network clients
type PacketClient interface {
	Start(chan<- *packet.Packet)
	Close()
}

// --- Constants for Layout ---
const (
	sidebarWidth = 20
	msgbarHeight = 7 // This is from our last change
)

// model holds the application's state
type model struct {
	width  int
	height int
	config config.Config // --- ADDED: Store config ---

	headerModel  header.Model
	mapModel     mapview.Model
	msgbarModel  msgbar.Model
	footerModel  footer.Model
	sidebarModel sidebar.Model

	packetClient PacketClient
	packetChan   chan *packet.Packet

	err error
}

// initialModel creates the starting model
func initialModel(conf config.Config, client PacketClient, pChan chan *packet.Packet) model {
	mapMod, err := mapview.New(mapShapePath, conf)
	if err != nil {
		return model{err: err}
	}

	headerMod := header.New()
	msgbarMod := msgbar.New()
	footerMod := footer.New(mapShapePath)
	sidebarMod := sidebar.New()

	footerMod.SetZoom(mapMod.GetZoomLevel())

	return model{
		width:        80, // Default width
		height:       60, // Default height
		config:       conf, // --- ADDED: Store config ---
		headerModel:  headerMod,
		mapModel:     mapMod,
		msgbarModel:  msgbarMod,
		footerModel:  footerMod,
		sidebarModel: sidebarMod,
		packetClient: client,
		packetChan:   pChan,
	}
}

// listenForPackets is a tea.Cmd that waits for the next packet
func (m model) listenForPackets() tea.Cmd {
	return func() tea.Msg {
		pkt := <-m.packetChan
		if pkt == nil {
			return fmt.Errorf("connection closed")
		}
		return pkt
	}
}

// --- NEW FUNCTION ---
// speakMessageCmd runs the 'say' command as a non-blocking side effect
func speakMessageCmd(msg string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("say", msg)
		// Use Start() for non-blocking. Run() would freeze the UI.
		// We ignore the error, but you could log it.
		cmd.Start()
		return nil // No message to send back to Update
	}
}

func (m model) Init() tea.Cmd {
	go m.packetClient.Start(m.packetChan)
	return m.listenForPackets()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.err != nil {
		if _, ok := msg.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}

	var (
		headerCmd  tea.Cmd
		mapCmd     tea.Cmd
		msgbarCmd  tea.Cmd
		footerCmd  tea.Cmd
		sidebarCmd tea.Cmd
		cmds       []tea.Cmd
	)

	switch msg := msg.(type) {
	case *packet.Packet:
		switch msg.Type {
		case packet.TypePosition:
			m.mapModel, mapCmd = m.mapModel.Update(msg)
			m.footerModel.SetLastPacket(msg.Callsign)
			m.sidebarModel.AddPacket(msg.Callsign)
			cmds = append(cmds, mapCmd)

		case packet.TypeMessage:
			// --- THIS IS THE NEW LOGIC ---
			if m.config.Msgbar.Say {
				// Format a more natural-sounding message for speech
				msgStr := fmt.Sprintf("Message from %s to %s: %s", msg.Callsign, msg.MsgTo, msg.MsgBody)
				cmds = append(cmds, speakMessageCmd(msgStr))
			}
			// --- END NEW LOGIC ---

			m.msgbarModel, msgbarCmd = m.msgbarModel.Update(msg)
			cmds = append(cmds, msgbarCmd)
		}
		cmds = append(cmds, m.listenForPackets())

	case error:
		m.err = msg
		log.Printf("Error received in Update: %v", msg)
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 1
		footerHeight := 1
		mainHeight := m.height - headerHeight - msgbarHeight - footerHeight
		mapWidth := m.width - sidebarWidth
		if mainHeight < 1 {
			mainHeight = 1
		}

		headerMsg := tea.WindowSizeMsg{Width: m.width, Height: headerHeight}
		m.headerModel, headerCmd = m.headerModel.Update(headerMsg)

		sidebarMsg := tea.WindowSizeMsg{Width: sidebarWidth, Height: mainHeight}
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(sidebarMsg)

		mapMsg := tea.WindowSizeMsg{Width: mapWidth, Height: mainHeight}
		m.mapModel, mapCmd = m.mapModel.Update(mapMsg)

		msgbarMsg := tea.WindowSizeMsg{Width: m.width, Height: msgbarHeight}
		m.msgbarModel, msgbarCmd = m.msgbarModel.Update(msgbarMsg)

		footerMsg := tea.WindowSizeMsg{Width: m.width, Height: footerHeight}
		m.footerModel, footerCmd = m.footerModel.Update(footerMsg)

		cmds = append(cmds, headerCmd, sidebarCmd, mapCmd, msgbarCmd, footerCmd)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		default:
			m.mapModel, mapCmd = m.mapModel.Update(msg)
			cmds = append(cmds, mapCmd)
			m.footerModel.SetZoom(m.mapModel.GetZoomLevel())
		}

	default:
		m.headerModel, headerCmd = m.headerModel.Update(msg)
		m.mapModel, mapCmd = m.mapModel.Update(msg)
		m.msgbarModel, msgbarCmd = m.msgbarModel.Update(msg)
		m.footerModel, footerCmd = m.footerModel.Update(msg)
		m.sidebarModel, sidebarCmd = m.sidebarModel.Update(msg)
		cmds = append(cmds, headerCmd, mapCmd, msgbarCmd, footerCmd, sidebarCmd)
	}

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
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

	headerView := m.headerModel.View()
	sidebarView := m.sidebarModel.View()
	mapView := m.mapModel.View()
	msgbarView := m.msgbarModel.View()
	footerView := m.footerModel.View()

	middleStack := lipgloss.JoinHorizontal(lipgloss.Top,
		sidebarView,
		mapView,
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		headerView,
		middleStack,
		msgbarView,
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
		// --- THIS IS THE FIX ---
		packetClient, connectErr = aprsis.Connect(conf)
		// --- END FIX ---
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