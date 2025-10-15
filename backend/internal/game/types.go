package game

import (
	"math"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
)

// StatUpgradeType defines the category of stat upgrade
type StatUpgradeType string

const (
	StatUpgradeHullStrength StatUpgradeType = "hullStrength" // Increases health and widens ship
	StatUpgradeAutoRepairs  StatUpgradeType = "autoRepairs"  // Health regeneration
	StatUpgradeCannonRange  StatUpgradeType = "cannonRange"  // Bullet speed and cannon length
	StatUpgradeCannonDamage StatUpgradeType = "cannonDamage" // Bullet damage and width
	StatUpgradeReloadSpeed  StatUpgradeType = "reloadSpeed"  // Reduces cooldown time
	StatUpgradeMoveSpeed    StatUpgradeType = "moveSpeed"    // Movement speed
	StatUpgradeTurnSpeed    StatUpgradeType = "turnSpeed"    // Turn rate
	StatUpgradeBodyDamage   StatUpgradeType = "bodyDamage"   // Collision damage
)

const maxPlayerNameLength = 16

var colorHexPattern = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

// StatUpgrade represents a single stat upgrade level
type StatUpgrade struct {
	Type        StatUpgradeType `json:"type"`
	Level       int             `json:"level"`       // Current level (0-75)
	MaxLevel    int             `json:"maxLevel"`    // Maximum level (75)
	BaseCost    int             `json:"baseCost"`    // Base cost (10)
	CurrentCost int             `json:"currentCost"` // Current upgrade cost
}

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
	UpgradeScatter   bool   `json:"upgradeScatter"`
	DowngradeScatter bool   `json:"downgradeScatter"`
	UpgradeTurrets   bool   `json:"upgradeTurrets"`
	DowngradeTurrets bool   `json:"downgradeTurrets"`
	// New leveling system inputs
	DebugLevelUp  bool   `json:"debugLevelUp"`
	SelectUpgrade string `json:"selectUpgrade"` // "side", "top", "front", "rear"
	UpgradeChoice string `json:"upgradeChoice"` // Specific upgrade ID/name
	// Stat upgrade inputs
	StatUpgradeType string `json:"statUpgradeType"` // Which stat to upgrade
	PlayerName      string `json:"playerName"`
	PlayerColor     string `json:"playerColor"`
	Mouse           struct {
		X float32 `json:"x"`
		Y float32 `json:"y"`
	} `json:"mouse"`
}

// Position represents the relative position of a single cannon from ship center
type Position struct {
	X float32 `json:"x"` // Relative X position from ship center
	Y float32 `json:"y"` // Relative Y position from ship center
}

// Player represents a game player
type Player struct {
	ID              uint32    `json:"id"`
	X               float32   `json:"x"`
	Y               float32   `json:"y"`
	VelX            float32   `json:"velX"`
	VelY            float32   `json:"velY"`
	Angle           float32   `json:"angle"` // Ship facing direction in radians
	AngularVelocity float32   `json:"-"`     // Current angular velocity (radians per update)
	Score           int       `json:"score"`
	State           int       `json:"state"`
	Name            string    `json:"name"`
	Color           string    `json:"color"`
	Health          int       `json:"health"`
	MaxHealth       int       `json:"maxHealth"`
	RespawnTime     time.Time `json:"-"` // When the player can respawn
	// Leveling system
	Level             int `json:"level"`             // Current player level
	Experience        int `json:"experience"`        // Current experience points
	AvailableUpgrades int `json:"availableUpgrades"` // Number of pending upgrade points
	// Category-specific reload times
	LastSideUpgradeShot  time.Time         `json:"-"`          // When side upgrades last fired
	LastTopUpgradeShot   time.Time         `json:"-"`          // When top upgrades last fired
	LastFrontUpgradeShot time.Time         `json:"-"`          // When front upgrades last fired
	LastRearUpgradeShot  time.Time         `json:"-"`          // When rear upgrades last fired
	ShipConfig           ShipConfiguration `json:"shipConfig"` // New modular upgrade system

	// Stat upgrades
	Coins               int                             `json:"coins"`        // Currency for stat upgrades
	StatUpgrades        map[StatUpgradeType]StatUpgrade `json:"statUpgrades"` // Applied stat upgrades
	LastRegenTime       time.Time                       `json:"-"`            // Last health regeneration time
	LastCollisionDamage time.Time                       `json:"-"`            // Last collision damage time
}

// GameItem represents collectible items in the game
type GameItem struct {
	ID    uint32  `json:"id"`
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Type  string  `json:"type"`
	Coins int     `json:"coins"`
	XP    int     `json:"xp"`
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
	Damage    int       `json:"damage"`
}

// Snapshot represents the current game state sent to clients
type Snapshot struct {
	Type    string     `json:"type"`
	Players []Player   `json:"players"`
	Items   []GameItem `json:"items"`
	Bullets []Bullet   `json:"bullets"`
	Time    int64      `json:"time"`
}

// WelcomeMsg represents a welcome message sent to a new client
type WelcomeMsg struct {
	Type     string `json:"type"`
	PlayerId uint32 `json:"playerId"`
}

// UpgradeInfo represents simplified upgrade information for client
type UpgradeInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// AvailableUpgradesMsg represents available upgrades for a player
type AvailableUpgradesMsg struct {
	Type     string                   `json:"type"`
	Upgrades map[string][]UpgradeInfo `json:"upgrades"`
}

// Client represents a connected game client
type Client struct {
	ID          uint32
	Conn        *websocket.Conn
	Player      *Player
	Input       InputMsg
	Send        chan []byte
	LastSeen    time.Time
	LastUpgrade time.Time // Prevents rapid upgrade applications
	mu          sync.RWMutex
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
	// Calculate initial shaft length (same logic as updateShipDimensions)
	shipLength := float32(PlayerSize*1.2) * 0.5 // Base shaft length for 1 cannon
	shipWidth := float32(PlayerSize * 0.8)

	shipConfig := ShipConfiguration{
		SideUpgrade:  NewSideUpgradeTree(),
		TopUpgrade:   NewTopUpgradeTree(),
		FrontUpgrade: nil,
		RearUpgrade:  nil,
		ShipLength:   shipLength,
		ShipWidth:    shipWidth,
		Size:         PlayerSize,
	}

	player := &Player{
		ID:                  id,
		X:                   WorldWidth / 2,
		Y:                   WorldHeight / 2,
		State:               StateAlive,
		Health:              100,
		MaxHealth:           100,
		Color:               generateRandomColor(),
		Name:                generateRandomName(),
		Level:               1,
		Experience:          0,
		AvailableUpgrades:   0,
		ShipConfig:          shipConfig,
		Coins:               100000, // Starting coins
		StatUpgrades:        make(map[StatUpgradeType]StatUpgrade),
		LastRegenTime:       time.Now(), // Initialize health regen timer
		LastCollisionDamage: time.Now(), // Initialize collision damage timer
	}

	// Initialize stat upgrades
	InitializeStatUpgrades(player)

	return player
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

// SanitizePlayerName cleans and bounds a requested player name.
func SanitizePlayerName(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	count := 0
	lastWasSpace := false

	for _, r := range trimmed {
		if count >= maxPlayerNameLength {
			break
		}

		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			builder.WriteRune(r)
			count++
			lastWasSpace = false
		case r == '\'' || r == '-':
			if builder.Len() == 0 || lastWasSpace {
				continue
			}
			builder.WriteRune(r)
			count++
			lastWasSpace = false
		case unicode.IsSpace(r):
			if !lastWasSpace && builder.Len() > 0 {
				builder.WriteRune(' ')
				count++
				lastWasSpace = true
			}
		default:
			continue
		}
	}

	result := strings.TrimSpace(builder.String())
	if result == "" {
		return ""
	}

	return result
}

// SanitizePlayerColor validates and normalises a requested hull colour.
func SanitizePlayerColor(input string) string {
	if input == "" {
		return ""
	}

	match := colorHexPattern.FindStringSubmatch(strings.TrimSpace(input))
	if len(match) != 2 {
		return ""
	}

	return "#" + strings.ToUpper(match[1])
}

// GetExperienceRequiredForLevel returns the experience needed to reach a specific level
func GetExperienceRequiredForLevel(level int) int {
	// Exponential progression: each level requires 50% more experience than the previous
	// Level 1 = 0, Level 2 = 100, Level 3 = 250, Level 4 = 475, etc.
	if level <= 1 {
		return 0
	}

	totalExp := 0
	baseExp := 100 // Experience needed to go from level 1 to 2

	for i := 2; i <= level; i++ {
		if i == 2 {
			totalExp += baseExp
		} else {
			// Each level requires 50% more than the previous level's requirement
			levelExp := int(float64(baseExp) * math.Pow(2, float64(i-2)))
			totalExp += levelExp
		}
	}

	return totalExp
}

// GetExperienceRequiredForNextLevel returns the experience needed to reach the next level
func (p *Player) GetExperienceRequiredForNextLevel() int {
	return GetExperienceRequiredForLevel(p.Level + 1)
}

// GetExperienceForCurrentLevel returns the experience required for the current level
func (p *Player) GetExperienceForCurrentLevel() int {
	return GetExperienceRequiredForLevel(p.Level)
}

// GetExperienceProgressToNextLevel returns progress (0.0 to 1.0) to next level
func (p *Player) GetExperienceProgressToNextLevel() float32 {
	currentLevelExp := p.GetExperienceForCurrentLevel()
	nextLevelExp := p.GetExperienceRequiredForNextLevel()
	levelExpNeeded := nextLevelExp - currentLevelExp

	if levelExpNeeded <= 0 {
		return 1.0
	}

	progress := float32(p.Experience-currentLevelExp) / float32(levelExpNeeded)
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		return 1
	}
	return progress
}

// AddExperience adds experience and handles level ups
func (p *Player) AddExperience(exp int) bool {
	p.Experience += exp

	// Check for level up
	if p.Experience >= p.GetExperienceRequiredForNextLevel() {
		p.Level++
		p.AvailableUpgrades++
		return true // Level up occurred
	}
	return false
}

// DebugLevelUp increases the player's level (for testing)
func (p *Player) DebugLevelUp() {
	p.Level++
	p.Experience = p.GetExperienceForCurrentLevel()
	p.AvailableUpgrades++
}

// InitializeStatUpgrades initializes the stat upgrade system for a player
func InitializeStatUpgrades(player *Player) {
	player.StatUpgrades = make(map[StatUpgradeType]StatUpgrade)

	upgradeTypes := []StatUpgradeType{
		StatUpgradeHullStrength,
		StatUpgradeAutoRepairs,
		StatUpgradeCannonRange,
		StatUpgradeCannonDamage,
		StatUpgradeReloadSpeed,
		StatUpgradeMoveSpeed,
		StatUpgradeTurnSpeed,
		StatUpgradeBodyDamage,
	}

	for _, upgradeType := range upgradeTypes {
		player.StatUpgrades[upgradeType] = StatUpgrade{
			Type:        upgradeType,
			Level:       0,
			MaxLevel:    15,
			BaseCost:    10,
			CurrentCost: 10,
		}
	}
}
