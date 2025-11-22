package game

// Game world constants
const (
	WorldWidth         = 5000.0
	WorldHeight        = 5000.0
	TickRate           = 30 // Server updates per second (reduced for performance)
	PlayerSize         = 50.0
	MaxPlayers         = 32
	BulletVisibleRange = 1500.0 // Maximum distance to send bullets to clients
)

// Ship physics constants
const (
	BaseShipTurnSpeed = 0.08 // Turning speed in radians per frame (doubled for 30 TPS)
	ShipDeceleration  = 0.84 // Drag/friction factor (adjusted for 30 TPS)
	BaseShipMaxSpeed  = 4    // Maximum speed (doubled for 30 TPS)
)

const (
	HealthIncrease = 30
)

// Cannon and bullet constants
const (
	BulletSpeed    = 12  // Bullet travel speed (doubled for 30 TPS)
	BulletLifetime = 2   // Seconds before bullet disappears
	BulletSize     = 8.0 // Bullet radius
	BulletDamage   = 6   // Damage per bullet hit (unchanged)
)

// Message types for client-server communication
const (
	MsgTypeSnapshot        = "snapshot"
	MsgTypeDeltaSnapshot   = "deltaSnapshot"
	MsgTypeWelcome         = "welcome"
	MsgTypeGameEvent       = "gameEvent"
	MsgTypeResetShipConfig = "resetShipConfig"
)

// Combat constants
const (
	BaseCollisionDamage = 5.0   // Base damage dealt per collision
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
	StateAlive = 0
	StateDead  = 1
)

const (
	DEV = false
)
