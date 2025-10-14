package game

// Game world constants
const (
	WorldWidth  = 2000.0
	WorldHeight = 2000.0
	TickRate    = 60 // Server updates per second
	PlayerSpeed = 2.0
	PlayerSize  = 50.0
	MaxPlayers  = 100
)

// Ship physics constants
const (
	ShipAcceleration  = 10000000 // Forward/backward acceleration
	BaseShipTurnSpeed = 0.04     // Turning speed in radians per frame
	ShipDeceleration  = 0.92     // Drag/friction factor
	BaseShipMaxSpeed  = 2        // Maximum speed
)

// Cannon and bullet constants
const (
	BulletSpeed         = 2    // Bullet travel speed (slower for easier tracking)
	BulletLifetime      = 2.5  // Seconds before bullet disappears (longer for easier spotting)
	BulletSize          = 8.0  // Bullet radius (much larger for visibility)
	BulletDamage        = 20   // Damage per bullet hit
	CannonCooldown      = 1    // Seconds between shots (faster for testing)
	CannonDistance      = 20.0 // Distance from ship center to cannon
	MaxCannonsPerSide   = 100  // Maximum cannons per side
	MinCannonsPerSide   = 1    // Minimum cannons per side
	CannonSpacingFactor = 0.7  // Factor for spacing cannons along ship length
	ShipScaleFactor     = 1.0  // Base scale factor for ship size
)

// Turret constants
const (
	TurretCooldown = 0.5 // Seconds between turret shots (faster than cannons)
	TurretRange    = 400 // Maximum turret firing range
	MaxTurrets     = 20  // Maximum number of turrets
	MinTurrets     = 0   // Minimum number of turrets
)

// Message types for client-server communication
const (
	MsgTypeInput    = "input"
	MsgTypeSnapshot = "snapshot"
	MsgTypeJoin     = "join"
	MsgTypeLeave    = "leave"
	MsgTypeScore    = "score"
	MsgTypeShoot    = "shoot"
	MsgTypeWelcome  = "welcome"
)

// Combat constants
const (
	RespawnDelay = 0.0 // Seconds to wait before respawning
)

// Item constants
const (
	ItemPickupSize = 16.0 // Size of item pickup bounding box
)

// Item type constants
const (
	ItemTypeGrayCircle   = "gray_circle"
	ItemTypeYellowCircle = "yellow_circle"
	ItemTypeOrangeCircle = "orange_circle"
	ItemTypeBlueDiamond  = "blue_diamond"
)

// Player states
const (
	StateAlive = iota
	StateDead
	StateSpawning
)
