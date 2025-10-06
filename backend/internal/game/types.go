package game

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// InputMsg represents player input from client
type InputMsg struct {
	Type             string `json:"type"`
	Up               bool   `json:"up"`
	Down             bool   `json:"down"`
	Left             bool   `json:"left"`
	Right            bool   `json:"right"`
	ShootLeft        bool   `json:"shootLeft"`
	ShootRight       bool   `json:"shootRight"`
	UpgradeCannons   bool   `json:"upgradeCannons"`
	DowngradeCannons bool   `json:"downgradeCannons"`
	Mouse            struct {
		X float32 `json:"x"`
		Y float32 `json:"y"`
	} `json:"mouse"`
}

// Player represents a game player
type Player struct {
	ID              uint32    `json:"id"`
	X               float32   `json:"x"`
	Y               float32   `json:"y"`
	VelX            float32   `json:"velX"`
	VelY            float32   `json:"velY"`
	Angle           float32   `json:"angle"` // Ship facing direction in radians
	Size            float32   `json:"size"`
	Score           int       `json:"score"`
	State           int       `json:"state"`
	Name            string    `json:"name"`
	Color           string    `json:"color"`
	Health          int       `json:"health"`
	MaxHealth       int       `json:"maxHealth"`
	LastShotTime    time.Time `json:"-"`
	CannonCount     int       `json:"cannonCount"`     // Number of cannons per side
	ShipLength      float32   `json:"shipLength"`      // Length of the ship
	ShipWidth       float32   `json:"shipWidth"`       // Width of the ship
	CollisionRadius float32   `json:"collisionRadius"` // Dynamic collision radius
}

// GameItem represents collectible items in the game
type GameItem struct {
	ID    uint32  `json:"id"`
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Type  string  `json:"type"`
	Value int     `json:"value"`
}

// Bullet represents a projectile fired from ship cannons
type Bullet struct {
	ID        uint32    `json:"id"`
	X         float32   `json:"x"`
	Y         float32   `json:"y"`
	VelX      float32   `json:"velX"`
	VelY      float32   `json:"velY"`
	OwnerID   uint32    `json:"ownerId"`
	CreatedAt time.Time `json:"-"`
	Size      float32   `json:"size"`
}

// Snapshot represents the current game state sent to clients
type Snapshot struct {
	Type    string     `json:"type"`
	Players []Player   `json:"players"`
	Items   []GameItem `json:"items"`
	Bullets []Bullet   `json:"bullets"`
	Time    int64      `json:"time"`
}

// Client represents a connected game client
type Client struct {
	ID       uint32
	Conn     *websocket.Conn
	Player   *Player
	Input    InputMsg
	Send     chan []byte
	LastSeen time.Time
	mu       sync.RWMutex
}

// World represents the game world and all its entities
type World struct {
	mu        sync.RWMutex
	clients   map[uint32]*Client
	players   map[uint32]*Player
	items     map[uint32]*GameItem
	bullets   map[uint32]*Bullet
	mechanics *GameMechanics
	nextID    uint32
	itemID    uint32
	bulletID  uint32
	running   bool
}

// NewClient creates a new client
func NewClient(id uint32, conn *websocket.Conn) *Client {
	return &Client{
		ID:       id,
		Conn:     conn,
		Player:   NewPlayer(id),
		Send:     make(chan []byte, 256),
		LastSeen: time.Now(),
	}
}

// NewPlayer creates a new player with default values
func NewPlayer(id uint32) *Player {
	cannonCount := 1 // Default number of cannons per side
	shipLength := float32(PlayerSize * 1.2)
	shipWidth := float32(PlayerSize * 0.8)

	return &Player{
		ID:              id,
		X:               WorldWidth / 2,
		Y:               WorldHeight / 2,
		Size:            PlayerSize,
		State:           StateAlive,
		Health:          100,
		MaxHealth:       100,
		Color:           generateRandomColor(),
		Name:            generateRandomName(),
		CannonCount:     cannonCount,
		ShipLength:      shipLength,
		ShipWidth:       shipWidth,
		CollisionRadius: calculateCollisionRadius(shipLength, shipWidth),
	}
}

// calculateCollisionRadius calculates the collision radius based on ship dimensions
func calculateCollisionRadius(length, width float32) float32 {
	// Use the larger dimension divided by 2 as the collision radius
	if length > width {
		return length / 2
	}
	return width / 2
}

func generateRandomColor() string {
	colors := []string{"#FF6B6B", "#4ECDC4", "#45B7D1", "#96CEB4", "#FFEAA7", "#DDA0DD", "#98D8C8", "#F7DC6F"}
	return colors[int(time.Now().UnixNano())%len(colors)]
}

func generateRandomName() string {
	names := []string{"Pirate", "Buccaneer", "Sailor", "Captain", "Admiral", "Navigator", "Corsair", "Raider"}
	return names[int(time.Now().UnixNano())%len(names)]
}
