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
	Type        UpgradeType `msgpack:"type"`
	Level       int         `msgpack:"level"`       // Current level (0-75)
	MaxLevel    int         `msgpack:"maxLevel"`    // Maximum level (75)
	BaseCost    int         `msgpack:"baseCost"`    // Base cost (10)
	CurrentCost int         `msgpack:"currentCost"` // Current upgrade cost
}

// InputMsg represents player input from client
type InputMsg struct {
	Type string `msgpack:"type"`
	// Movement inputs (continuous state)
	Up    bool `msgpack:"up"`
	Down  bool `msgpack:"down"`
	Left  bool `msgpack:"left"`
	Right bool `msgpack:"right"`
	// Action inputs (single-fire events with sequence numbers)
	Actions []InputAction `msgpack:"actions,omitempty"`
	// Mouse position
	Mouse struct {
		X float64 `msgpack:"x"`
		Y float64 `msgpack:"y"`
	} `msgpack:"mouse"`
	// Legacy inputs (deprecated but kept for compatibility)
	UpgradeCannons   bool   `msgpack:"upgradeCannons,omitempty"`
	DowngradeCannons bool   `msgpack:"downgradeCannons,omitempty"`
	UpgradeTurrets   bool   `msgpack:"upgradeTurrets,omitempty"`
	DowngradeTurrets bool   `msgpack:"downgradeTurrets,omitempty"`
	DebugLevelUp     bool   `msgpack:"debugLevelUp,omitempty"`
	SelectUpgrade    string `msgpack:"selectUpgrade,omitempty"`
	UpgradeChoice    string `msgpack:"upgradeChoice,omitempty"`
	StatUpgradeType  string `msgpack:"statUpgradeType,omitempty"`
	ToggleAutofire   bool   `msgpack:"toggleAutofire,omitempty"`
	ManualFire       bool   `msgpack:"manualFire,omitempty"`
	RequestRespawn   bool   `msgpack:"requestRespawn,omitempty"`
	StartGame        bool   `msgpack:"startGame,omitempty"`
	PlayerName       string `msgpack:"playerName,omitempty"`
	PlayerColor      string `msgpack:"playerColor,omitempty"`
}

// InputAction represents a single-fire action with deduplication
type InputAction struct {
	Type     string `msgpack:"type"`     // "statUpgrade", "toggleAutofire", etc.
	Sequence uint32 `msgpack:"sequence"` // Client-side sequence number for deduplication
	Data     string `msgpack:"data"`     // Action-specific data (e.g., stat type for upgrades)
}

// Position represents the relative position of a single cannon from ship center
type Position struct {
	X float64 `msgpack:"x"` // Relative X position from ship center
	Y float64 `msgpack:"y"` // Relative Y position from ship center
}

// DebugInfo contains calculated debug values for client display
type DebugInfo struct {
	Health            int     `msgpack:"health"`
	MoveSpeedModifier float64 `msgpack:"moveSpeedModifier"`
	TurnSpeedModifier float64 `msgpack:"turnSpeedModifier"`
	RegenRate         float64 `msgpack:"regenRate"`
	BodyDamage        float64 `msgpack:"bodyDamage"`
	FrontDPS          float64 `msgpack:"frontDps"`
	SideDPS           float64 `msgpack:"sideDps"`
	RearDPS           float64 `msgpack:"rearDps"`
	TopDPS            float64 `msgpack:"topDps"`
	TotalDPS          float64 `msgpack:"totalDps"`
}

// Player represents a game player
type Player struct {
	ID          uint32    `msgpack:"id"`
	X           float64   `msgpack:"x"`
	Y           float64   `msgpack:"y"`
	VelX        float64   `msgpack:"velX"`
	VelY        float64   `msgpack:"velY"`
	Angle       float64   `msgpack:"angle"` // Ship facing direction in radians
	Score       int       `msgpack:"score"`
	State       int       `msgpack:"state"`
	Name        string    `msgpack:"name"`
	Color       string    `msgpack:"color"`
	IsBot       bool      `msgpack:"isBot"`
	Health      int       `msgpack:"health"`
	MaxHealth   int       `msgpack:"maxHealth"`
	RespawnTime time.Time `msgpack:"-"` // When the player can respawn

	Client *Client `msgpack:"-"` // Back-reference to owning client (not serialized)
	// Leveling system
	Level             int `msgpack:"level"`             // Current player level
	Experience        int `msgpack:"experience"`        // Current experience points
	AvailableUpgrades int `msgpack:"availableUpgrades"` // Number of pending upgrade points
	// Category-specific reload times
	LastSideUpgradeShot  time.Time         `msgpack:"-"`          // When side upgrades last fired
	LastTopUpgradeShot   time.Time         `msgpack:"-"`          // When top upgrades last fired
	LastFrontUpgradeShot time.Time         `msgpack:"-"`          // When front upgrades last fired
	LastRearUpgradeShot  time.Time         `msgpack:"-"`          // When rear upgrades last fired
	ShipConfig           ShipConfiguration `msgpack:"shipConfig"` // New modular upgrade system

	// Stat upgrades
	Coins     int                     `msgpack:"coins"`        // Currency for stat upgrades
	Upgrades  map[UpgradeType]Upgrade `msgpack:"statUpgrades"` // Applied stat upgrades
	Modifiers Mods                    `msgpack:"-"`            // Calculated stat modifiers (not serialized)

	LastRegenTime       time.Time `msgpack:"-"` // Last health regeneration time
	LastCollisionDamage time.Time `msgpack:"-"` // Last collision damage time
	// Autofire toggle state
	AutofireEnabled bool `msgpack:"autofireEnabled"` // Whether autofire is currently enabled
	// Action processing state (for deduplication)
	LastProcessedAction uint32               `msgpack:"-"` // Last processed action sequence number
	ActionCooldowns     map[string]time.Time `msgpack:"-"` // Cooldowns per action type
	// Death tracking
	KilledBy     uint32    `msgpack:"killedBy"`     // ID of player who killed this player (0 if none)
	KilledByName string    `msgpack:"killedByName"` // Name of player who killed this player
	DeathTime    time.Time `msgpack:"-"`            // When the player died
	ScoreAtDeath int       `msgpack:"scoreAtDeath"` // Score when player died
	SurvivalTime float64   `msgpack:"survivalTime"` // How long the player was alive (in seconds)
	SpawnTime    time.Time `msgpack:"-"`            // When the player spawned
	DebugInfo    DebugInfo `msgpack:"debugInfo"`    // Calculated debug values for client
}

// Bot wraps an AI-controlled player with simple state required for decision making.
type Bot struct {
	ID                uint32
	Player            *Player
	Input             InputMsg
	GuardCenter       Position
	GuardRadius       float64
	AggroRadius       float64
	TargetDistance    float64
	PreferredDistance float64
	NextDecision      time.Time
	TargetPlayerID    uint32
	OrbitDirection    int
	TurnIntent        float64
	DesiredAngle      float64
}

// GameItem represents collectible items in the game
type GameItem struct {
	ID    uint32  `msgpack:"id"`
	X     float64 `msgpack:"x"`
	Y     float64 `msgpack:"y"`
	Type  string  `msgpack:"type"`
	Coins int     `msgpack:"coins"`
	XP    int     `msgpack:"xp"`
}

// Bullet represents a projectile fired from ship cannons
type Bullet struct {
	ID        uint32    `msgpack:"id"`
	X         float64   `msgpack:"x"`
	Y         float64   `msgpack:"y"`
	VelX      float64   `msgpack:"velX"`
	VelY      float64   `msgpack:"velY"`
	OwnerID   uint32    `msgpack:"ownerId"`
	CreatedAt time.Time `msgpack:"-"` // Not serialized
	Size      float64   `msgpack:"size"`
	Damage    int       `msgpack:"damage"`
}

// Snapshot represents the current game state sent to clients
type Snapshot struct {
	Type    string     `msgpack:"type"`
	Players []Player   `msgpack:"players"`
	Items   []GameItem `msgpack:"items"`
	Bullets []Bullet   `msgpack:"bullets"`
	Time    int64      `msgpack:"time"`
}

// DeltaSnapshot represents only the changes in game state since last snapshot
type DeltaSnapshot struct {
	Type         string        `msgpack:"type"`
	Players      []PlayerDelta `msgpack:"players,omitempty"`      // Delta player updates
	ItemsAdded   []GameItem    `msgpack:"itemsAdded,omitempty"`   // Items that were added
	ItemsRemoved []uint32      `msgpack:"itemsRemoved,omitempty"` // IDs of items that were removed
	Bullets      []Bullet      `msgpack:"bullets,omitempty"`      // Full bullet list (always sent)
	Time         int64         `msgpack:"time"`
}

// PlayerDelta represents only the changed fields of a player since last snapshot
type PlayerDelta struct {
	ID                uint32                   `msgpack:"id"`          // Always sent
	X                 *float64                 `msgpack:"x,omitempty"` // Position changes frequently
	Y                 *float64                 `msgpack:"y,omitempty"`
	VelX              *float64                 `msgpack:"velX,omitempty"`
	VelY              *float64                 `msgpack:"velY,omitempty"`
	Angle             *float64                 `msgpack:"angle,omitempty"`
	Score             *int                     `msgpack:"score,omitempty"`             // Changes occasionally
	State             *int                     `msgpack:"state,omitempty"`             // Alive/dead state
	Name              *string                  `msgpack:"name,omitempty"`              // Changes rarely
	Color             *string                  `msgpack:"color,omitempty"`             // Changes rarely
	Health            *int                     `msgpack:"health,omitempty"`            // Changes frequently
	MaxHealth         *int                     `msgpack:"maxHealth,omitempty"`         // Changes with upgrades
	Level             *int                     `msgpack:"level,omitempty"`             // Changes occasionally
	Experience        *int                     `msgpack:"experience,omitempty"`        // Changes frequently
	AvailableUpgrades *int                     `msgpack:"availableUpgrades,omitempty"` // Changes occasionally
	ShipConfig        ShipConfigDelta          `msgpack:"shipConfig"`                  // Always sent (minimal data for rendering)
	Coins             *int                     `msgpack:"coins,omitempty"`             // Changes with items/spending
	Upgrades          *map[UpgradeType]Upgrade `msgpack:"statUpgrades,omitempty"`      // Changes with stat upgrades
	AutofireEnabled   *bool                    `msgpack:"autofireEnabled,omitempty"`   // Changes rarely
	DebugInfo         *DebugInfo               `msgpack:"debugInfo,omitempty"`         // Changes frequently for display
}

// ShipConfigDelta contains only the fields needed by the frontend for rendering
type ShipConfigDelta struct {
	ShipLength   float64          `msgpack:"shipLength,omitempty"`   // For hull dimensions
	ShipWidth    float64          `msgpack:"shipWidth,omitempty"`    // For hull dimensions
	SideUpgrade  *ShipModuleDelta `msgpack:"sideUpgrade,omitempty"`  // Side cannons
	FrontUpgrade *ShipModuleDelta `msgpack:"frontUpgrade,omitempty"` // Front upgrades (ram/cannons)
	RearUpgrade  *ShipModuleDelta `msgpack:"rearUpgrade,omitempty"`  // Rear upgrades (rudder)
	TopUpgrade   *ShipModuleDelta `msgpack:"topUpgrade,omitempty"`   // Top turrets
}

// ShipModuleDelta contains only the fields needed by the frontend
type ShipModuleDelta struct {
	Name    string        `msgpack:"name"`              // Upgrade name (for ram/rudder)
	Cannons []CannonDelta `msgpack:"cannons,omitempty"` // Cannons with minimal data
	Turrets []TurretDelta `msgpack:"turrets,omitempty"` // Turrets with minimal data
}

// CannonDelta contains only the fields needed by the frontend for rendering
type CannonDelta struct {
	Position   Position  `msgpack:"position,omitempty"`   // Relative position for drawing
	Type       string    `msgpack:"type,omitempty"`       // Cannon type for rendering style
	RecoilTime time.Time `msgpack:"recoilTime,omitempty"` // For recoil animation
}

// TurretDelta contains only the fields needed by the frontend for rendering
type TurretDelta struct {
	Position        Position      `msgpack:"position,omitempty"`        // Relative position for drawing
	Angle           float64       `msgpack:"angle,omitempty"`           // Current aiming angle
	Type            string        `msgpack:"type,omitempty"`            // Turret type for rendering style
	RecoilTime      time.Time     `msgpack:"recoilTime,omitempty"`      // For recoil animation
	NextCannonIndex int           `msgpack:"nextCannonIndex,omitempty"` // For alternating recoil
	Cannons         []CannonDelta `msgpack:"cannons,omitempty"`         // Turret cannons (minimal data)
}

// WelcomeMsg represents a welcome message sent to a new client
type WelcomeMsg struct {
	Type     string `msgpack:"type"`
	PlayerId uint32 `msgpack:"playerId"`
}

// UpgradeInfo represents simplified upgrade information for client
type UpgradeInfo struct {
	Name string `msgpack:"name"`
	Type string `msgpack:"type"`
}

// AvailableUpgradesMsg represents available upgrades for a player
type AvailableUpgradesMsg struct {
	Type     string                   `msgpack:"type"`
	Upgrades map[string][]UpgradeInfo `msgpack:"upgrades"`
}

// GameEventMsg represents a one-off gameplay notification
type GameEventMsg struct {
	Type       string `msgpack:"type"`
	EventType  string `msgpack:"eventType"`
	KillerID   uint32 `msgpack:"killerId,omitempty"`
	KillerName string `msgpack:"killerName,omitempty"`
	VictimID   uint32 `msgpack:"victimId,omitempty"`
	VictimName string `msgpack:"victimName,omitempty"`
}

// Client represents a connected game client
type Client struct {
	ID           uint32
	Conn         *websocket.Conn
	Player       *Player
	Input        InputMsg
	Send         chan []byte
	LastSeen     time.Time
	LastUpgrade  time.Time // Prevents rapid upgrade applications
	lastSnapshot Snapshot  // Store the last sent snapshot for delta calculations
	mu           sync.RWMutex
}

// World represents the game world and all its entities
type World struct {
	mu                sync.RWMutex
	clients           map[uint32]*Client
	players           map[uint32]*Player
	bots              map[uint32]*Bot
	items             map[uint32]*GameItem
	bullets           map[uint32]*Bullet
	mechanics         *GameMechanics
	nextPlayerID      uint32
	itemID            uint32
	bulletID          uint32
	running           bool
	tickCounter       uint32 // For performance optimizations
	botsSpawned       bool
	snapshotCount     int64 // Total snapshots sent
	totalSnapshotSize int64 // Total size of all snapshots
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
	shipLength := float64(PlayerSize*1.2) * 0.5 // Base shaft length for 1 cannon
	shipWidth := float64(PlayerSize * 0.8)

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
func (p *Player) GetExperienceProgressToNextLevel() float64 {
	currentLevelExp := p.GetExperienceForCurrentLevel()
	nextLevelExp := p.GetExperienceRequiredForNextLevel()
	levelExpNeeded := nextLevelExp - currentLevelExp

	if levelExpNeeded <= 0 {
		return 1.0
	}

	progress := float64(p.Experience-currentLevelExp) / float64(levelExpNeeded)
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
