package server

import (
	"encoding/json"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"goblons/internal/game"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connections from any origin
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Server handles HTTP and WebSocket connections
type Server struct {
	world         *game.World
	bytesSent     int64 // Total bytes sent
	bytesReceived int64 // Total bytes received
	messagesSent  int64 // Total messages sent
	messagesRecv  int64 // Total messages received
}

// NewServer creates a new server instance
func NewServer() *Server {
	server := &Server{
		world: game.NewWorld(),
	}
	
	// Start network monitoring
	go server.monitorNetworkUsage()
	
	return server
}

// Start starts the server on the specified address
func (s *Server) Start(addr string) error {
	// Start the game world
	go s.world.Start()

	// Set up HTTP routes
	http.Handle("/", http.FileServer(http.Dir("./static")))
	http.HandleFunc("/ws", s.handleWebSocket)

	log.Printf("Server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// monitorNetworkUsage logs network statistics every 10 seconds
func (s *Server) monitorNetworkUsage() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastSent, lastRecv int64
	var lastMsgSent, lastMsgRecv int64
	
	for range ticker.C {
		currentSent := atomic.LoadInt64(&s.bytesSent)
		currentRecv := atomic.LoadInt64(&s.bytesReceived)
		currentMsgSent := atomic.LoadInt64(&s.messagesSent)
		currentMsgRecv := atomic.LoadInt64(&s.messagesRecv)
		
		sentRate := float64(currentSent-lastSent) / 10.0
		recvRate := float64(currentRecv-lastRecv) / 10.0
		msgSentRate := float64(currentMsgSent-lastMsgSent) / 10.0
		msgRecvRate := float64(currentMsgRecv-lastMsgRecv) / 10.0
		
		log.Printf("Network Stats - Sent: %.1f B/s (%d total), Recv: %.1f B/s (%d total), Msg Sent: %.1f/s (%d total), Msg Recv: %.1f/s (%d total)",
			sentRate, currentSent, recvRate, currentRecv, msgSentRate, currentMsgSent, msgRecvRate, currentMsgRecv)
		
		lastSent = currentSent
		lastRecv = currentRecv
		lastMsgSent = currentMsgSent
		lastMsgRecv = currentMsgRecv
	}
}

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create new client
	client := game.NewClient(0, conn) // ID will be assigned by world

	// Apply any requested cosmetics before joining the world
	query := r.URL.Query()
	if requestedName := game.SanitizePlayerName(query.Get("name")); requestedName != "" {
		client.Player.Name = requestedName
	}
	if requestedColor := game.SanitizePlayerColor(query.Get("color")); requestedColor != "" {
		client.Player.Color = requestedColor
	}

	// Try to add client (may fail if server is full)
	if !s.world.AddClient(client) {
		// Server is full, send error and close connection
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "Server is full"))
		conn.Close()
		return
	}

	// Start client goroutines
	go s.handleClientReads(client)
	go s.handleClientWrites(client)
}

// handleClientReads reads messages from the client
func (s *Server) handleClientReads(client *game.Client) {
	defer func() {
		client.Conn.Close()
		s.world.RemoveClient(client.ID)
	}()

	// Set read deadline and pong handler for keepalive
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, messageBytes, err := client.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Track received bytes and messages
		atomic.AddInt64(&s.bytesReceived, int64(len(messageBytes)))
		atomic.AddInt64(&s.messagesRecv, 1)

		var input game.InputMsg
		if err := json.Unmarshal(messageBytes, &input); err != nil {
			log.Printf("Error unmarshaling input: %v", err)
			continue
		}

		// Process the input
		s.world.HandleInput(client.ID, input)
	}
}

// handleClientWrites sends messages to the client
func (s *Server) handleClientWrites(client *game.Client) {
	ticker := time.NewTicker(54 * time.Second) // Send ping every 54 seconds
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Track sent bytes and messages
			atomic.AddInt64(&s.bytesSent, int64(len(message)))
			atomic.AddInt64(&s.messagesSent, 1)

			if err := client.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("Write error: %v", err)
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
