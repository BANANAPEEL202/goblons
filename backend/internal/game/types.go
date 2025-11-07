package game

import (
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gorilla/websocket"
)

// UpgradeType defines the category of stat upgrade
type UpgradeType string

const (
	StatUpgradeHullStrength UpgradeType = "hullStrength" // Increases health and widens ship
	StatUpgradeAutoRepairs  UpgradeType = "autoRepairs"  // Health regeneration
	StatUpgradeCannonRange  UpgradeType = "cannonRange"  // Bullet speed and cannon length
	StatUpgradeCannonDamage UpgradeType = "cannonDamage" // Bullet damage and width
	StatUpgradeReloadSpeed  UpgradeType = "reloadSpeed"  // Reduces cooldown time
	StatUpgradeMoveSpeed    UpgradeType = "moveSpeed"    // Movement speed
	StatUpgradeTurnSpeed    UpgradeType = "turnSpeed"    // Turn rate
	StatUpgradeBodyDamage   UpgradeType = "bodyDamage"   // Collision damage
)

const maxPlayerNameLength = 16

var colorHexPattern = regexp.MustCompile(`^#?([0-9a-fA-F]{6})$`)

// Upgrade represents a single stat upgrade level
type Upgrade struct {
	Type        UpgradeType `json:"type"`
	Level       int         `json:"level"`       // Current level (0-75)
	MaxLevel    int         `json:"maxLevel"`    // Maximum level (75)
	BaseCost    int         `json:"baseCost"`    // Base cost (10)
	CurrentCost int         `json:"currentCost"` // Current upgrade cost
}

// InputMsg represents player input from client
type InputMsg struct {
	Type string `json:"type"`
	// Movement inputs (continuous state)
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
	// Action inputs (single-fire events with sequence numbers)
	Actions []InputAction `json:"actions,omitempty"`
	// Mouse position
	Mouse struct {
		X float32 `json:"x"`
		Y float32 `json:"y"`
	} `json:"mouse"`
	// Legacy inputs (deprecated but kept for compatibility)
	UpgradeCannons   bool   `json:"upgradeCannons,omitempty"`
	DowngradeCannons bool   `json:"downgradeCannons,omitempty"`
	UpgradeScatter   bool   `json:"upgradeScatter,omitempty"`
	DowngradeScatter bool   `json:"downgradeScatter,omitempty"`
	UpgradeTurrets   bool   `json:"upgradeTurrets,omitempty"`
	DowngradeTurrets bool   `json:"downgradeTurrets,omitempty"`
	DebugLevelUp     bool   `json:"debugLevelUp,omitempty"`
	SelectUpgrade    string `json:"selectUpgrade,omitempty"`
	UpgradeChoice    string `json:"upgradeChoice,omitempty"`
	StatUpgradeType  string `json:"statUpgradeType,omitempty"`
	ToggleAutofire   bool   `json:"toggleAutofire,omitempty"`
	ManualFire       bool   `json:"manualFire,omitempty"`
	RequestRespawn   bool   `json:"requestRespawn,omitempty"`
	StartGame        bool   `json:"startGame,omitempty"`
	PlayerName       string `json:"playerName,omitempty"`
	PlayerColor      string `json:"playerColor,omitempty"`
}

// InputAction represents a single-fire action with deduplication
type InputAction struct {
	Type     string `json:"type"`     // "statUpgrade", "toggleAutofire", etc.
	Sequence uint32 `json:"sequence"` // Client-side sequence number for deduplication
	Data     string `json:"data"`     // Action-specific data (e.g., stat type for upgrades)
}

// Position represents the relative position of a single cannon from ship center
type Position struct {
	X float32 `json:"x"` // Relative X position from ship center
	Y float32 `json:"y"` // Relative Y position from ship center
}

// DebugInfo contains calculated debug values for client display
type DebugInfo struct {
	Health            int     `json:"health"`
	MoveSpeedModifier float32 `json:"moveSpeedModifier"`
	TurnSpeedModifier float32 `json:"turnSpeedModifier"`
	RegenRate         float32 `json:"regenRate"`
	BodyDamage        float32 `json:"bodyDamage"`
	FrontDPS          float32 `json:"frontDps"`
	SideDPS           float32 `json:"sideDps"`
	RearDPS           float32 `json:"rearDps"`
	TopDPS            float32 `json:"topDps"`
	TotalDPS          float32 `json:"totalDps"`
}

// Player represents a game player
type Player struct {
	ID          uint32    `json:"id"`
	X           float32   `json:"x"`
	Y           float32   `json:"y"`
	VelX        float32   `json:"velX"`
	VelY        float32   `json:"velY"`
	Angle       float32   `json:"angle"` // Ship facing direction in radians
	Score       int       `json:"score"`
	State       int       `json:"state"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	IsBot       bool      `json:"isBot"`
	Health      int       `json:"health"`
	MaxHealth   int       `json:"maxHealth"`
	RespawnTime time.Time `json:"-"` // When the player can respawn

	Client *Client `json:"-"` // Back-reference to owning client (not serialized)
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
	Coins     int                     `json:"coins"`        // Currency for stat upgrades
	Upgrades  map[UpgradeType]Upgrade `json:"statUpgrades"` // Applied stat upgrades
	Modifiers Mods                    `json:"-"`            // Calculated stat modifiers (not serialized)

	LastRegenTime       time.Time `json:"-"` // Last health regeneration time
	LastCollisionDamage time.Time `json:"-"` // Last collision damage time
	// Autofire toggle state
	AutofireEnabled bool `json:"autofireEnabled"` // Whether autofire is currently enabled
	// Action processing state (for deduplication)
	LastProcessedAction uint32               `json:"-"` // Last processed action sequence number
	ActionCooldowns     map[string]time.Time `json:"-"` // Cooldowns per action type
	// Death tracking
	KilledBy     uint32    `json:"killedBy"`     // ID of player who killed this player (0 if none)
	KilledByName string    `json:"killedByName"` // Name of player who killed this player
	DeathTime    time.Time `json:"-"`            // When the player died
	ScoreAtDeath int       `json:"scoreAtDeath"` // Score when player died
	SurvivalTime float64   `json:"survivalTime"` // How long the player was alive (in seconds)
	SpawnTime    time.Time `json:"-"`            // When the player spawned
	DebugInfo    DebugInfo `json:"debugInfo"`    // Calculated debug values for client
}

// Bot wraps an AI-controlled player with simple state required for decision making.
type Bot struct {
	ID                uint32
	Player            *Player
	Input             InputMsg
	GuardCenter       Position
	GuardRadius       float32
	AggroRadius       float32
	TargetDistance    float32
	PreferredDistance float32
	NextDecision      time.Time
	TargetPlayerID    uint32
	OrbitDirection    int
	TurnIntent        float32
	DesiredAngle      float32
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

// DeltaSnapshot represents only the changes in game state since last snapshot
type DeltaSnapshot struct {
	Type       string     `json:"type"`
	Players    []Player   `json:"players,omitempty"`    // Full player list (always sent)
	ItemsAdded []GameItem `json:"itemsAdded,omitempty"` // Items that were added
	ItemsRemoved []uint32 `json:"itemsRemoved,omitempty"` // IDs of items that were removed
	Bullets    []Bullet   `json:"bullets,omitempty"`    // Full bullet list (always sent)
	Time       int64      `json:"time"`
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

// GameEventMsg represents a one-off gameplay notification
type GameEventMsg struct {
	Type       string `json:"type"`
	EventType  string `json:"eventType"`
	KillerID   uint32 `json:"killerId,omitempty"`
	KillerName string `json:"killerName,omitempty"`
	VictimID   uint32 `json:"victimId,omitempty"`
	VictimName string `json:"victimName,omitempty"`
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
	lastSnapshot Snapshot // Store the last sent snapshot for delta calculations
	mu          sync.RWMutex
}

// World represents the game world and all its entities
type World struct {
	mu           sync.RWMutex
	clients      map[uint32]*Client
	players      map[uint32]*Player
	bots         map[uint32]*Bot
	items        map[uint32]*GameItem
	bullets      map[uint32]*Bullet
	mechanics    *GameMechanics
	nextPlayerID uint32
	itemID       uint32
	bulletID     uint32
	running      bool
	tickCounter  uint32 // For performance optimizations
	botsSpawned  bool
}

// NewClient creates a new client
func NewClient(id uint32, conn *websocket.Conn) *Client {
	player := NewPlayer(id)
	client := &Client{
		ID:       id,
		Conn:     conn,
		Player:   player,
		Send:     make(chan []byte, 256),
		LastSeen: time.Now(),
	}
	player.Client = client
	return client
}

// NewPlayer creates a new player with default values
func NewPlayer(id uint32) *Player {
	// Calculate initial shaft length (same logic as updateShipDimensions)
	shipLength := float32(PlayerSize*1.2) * 0.5 // Base shaft length for 1 cannon
	shipWidth := float32(PlayerSize * 0.8)

	shipConfig := ShipConfiguration{
		SideUpgrade:  NewSideUpgradeTree(),
		TopUpgrade:   NewTopUpgradeTree(),
		FrontUpgrade: NewFrontUpgradeTree(),
		RearUpgrade:  NewRearUpgradeTree(),
		ShipLength:   shipLength,
		ShipWidth:    shipWidth,
		Size:         PlayerSize,
	}

	mods := Mods{
		SpeedMultiplier:        1.0,
		HealthRegenPerSec:      1.0,
		BulletSpeedMultiplier:  1.0,
		BulletDamageMultiplier: 1.0,
		ReloadSpeedMultiplier:  1.0,
		MoveSpeedMultiplier:    1.0,
		TurnSpeedMultiplier:    1.0,
		BodyDamageBonus:        1.0,
	}

	player := &Player{
		ID:                  id,
		X:                   WorldWidth / 2,
		Y:                   WorldHeight / 2,
		State:               StateAlive,
		Health:              100,
		MaxHealth:           100,
		Modifiers:           mods,
		Color:               generateRandomColor(),
		Name:                generateRandomName(),
		Level:               1,
		Experience:          0,
		AvailableUpgrades:   0,
		ShipConfig:          shipConfig,
		Coins:               0, // Starting coins
		Upgrades:            make(map[UpgradeType]Upgrade),
		LastRegenTime:       time.Now(),                 // Initialize health regen timer
		LastProcessedAction: 0,                          // No actions processed yet
		ActionCooldowns:     make(map[string]time.Time), // Initialize cooldown map
		LastCollisionDamage: time.Now(),                 // Initialize collision damage timer
	}

	// Initialize stat upgrades
	InitializeStatUpgrades(player)

	return player
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
	// Progressive increment: each level requires 100 more XP than the previous level's increment
	// Level 1 = 0, Level 2 = 100, Level 3 = 300, Level 4 = 600, Level 5 = 1000, etc.
	if level <= 1 {
		return 0
	}

	totalExp := 0

	for i := 2; i <= level; i++ {
		// Level increment increases by 100 each level: 100, 200, 300, 400...
		levelIncrement := (i - 1) * 100
		totalExp += levelIncrement
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
func (p *Player) AddExperience(exp int) {
	p.Experience += exp

	// Check for level up
	if p.Experience >= p.GetExperienceRequiredForNextLevel() {
		p.Level++
		p.AvailableUpgrades++
	}
}

// DebugLevelUp increases the player's level (for testing)
func (p *Player) DebugLevelUp() {
	p.Level++
	p.Experience = p.GetExperienceForCurrentLevel()
	p.AvailableUpgrades++
}

// InitializeStatUpgrades initializes the stat upgrade system for a player
func InitializeStatUpgrades(player *Player) {
	player.Upgrades = make(map[UpgradeType]Upgrade)

	upgradeTypes := []UpgradeType{
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
		player.Upgrades[upgradeType] = Upgrade{
			Type:        upgradeType,
			Level:       0,
			MaxLevel:    15,
			BaseCost:    10,
			CurrentCost: 10,
		}
	}
}
