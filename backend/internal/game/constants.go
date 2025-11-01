package game

// Game world constants
const (
	WorldWidth  = 3000 //5000.0
	WorldHeight = 3000 //5000.0
	TickRate    = 30   // Server updates per second (reduced for performance)
	PlayerSpeed = 2.0
	PlayerSize  = 50.0
	MaxPlayers  = 32
)

// Ship physics constants
const (
	ShipAcceleration  = 1000 // Forward/backward acceleration (doubled for 30 TPS)
	BaseShipTurnSpeed = 0.08 // Turning speed in radians per frame (doubled for 30 TPS)
	ShipDeceleration  = 0.84 // Drag/friction factor (adjusted for 30 TPS)
	BaseShipMaxSpeed  = 4    // Maximum speed (doubled for 30 TPS)
)

// Cannon and bullet constants
const (
	BulletSpeed    = 12  // Bullet travel speed (doubled for 30 TPS)
	BulletLifetime = 2   // Seconds before bullet disappears (unchanged - time-based)
	BulletSize     = 8.0 // Bullet radius (unchanged - visual)
	BulletDamage   = 8   // Damage per bullet hit (unchanged)
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
	RespawnDelay        = 0.0 // Seconds to wait before respawning
	BaseCollisionDamage = 5   // Base damage dealt per collision
	CollisionCooldown   = 0.2 // Seconds between collision damage ticks
)

// Item constants
const (
	ItemPickupSize = 16.0 // Size of item pickup bounding box
	MaxItems       = 300  // Maximum number of items in the world
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
