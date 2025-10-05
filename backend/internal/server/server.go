package server

import (
	"encoding/json"
	"log"
	"net/http"
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
	world *game.World
}

// NewServer creates a new server instance
func NewServer() *Server {
	return &Server{
		world: game.NewWorld(),
	}
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

// handleWebSocket handles WebSocket connections
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create new client
	client := game.NewClient(0, conn) // ID will be assigned by world
	s.world.AddClient(client)

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
