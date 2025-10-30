package game

// Game world constants
const (
	WorldWidth  = 2000.0
	WorldHeight = 2000.0
	TickRate    = 30 // Server updates per second (reduced for performance)
	PlayerSpeed = 2.0
	PlayerSize  = 50.0
	MaxPlayers  = 32
)

// Ship physics constants
const (
	ShipAcceleration  = 20000000 // Forward/backward acceleration (doubled for 30 TPS)
	BaseShipTurnSpeed = 0.08     // Turning speed in radians per frame (doubled for 30 TPS)
	ShipDeceleration  = 0.84     // Drag/friction factor (adjusted for 30 TPS)
	BaseShipMaxSpeed  = 4        // Maximum speed (doubled for 30 TPS)
)

// Cannon and bullet constants
const (
	BulletSpeed         = 8    // Bullet travel speed (doubled for 30 TPS)
	BulletLifetime      = 2.5  // Seconds before bullet disappears (unchanged - time-based)
	BulletSize          = 8.0  // Bullet radius (unchanged - visual)
	BulletDamage        = 8    // Damage per bullet hit (unchanged)
	CannonCooldown      = 1.5  // Seconds between shots (unchanged - time-based)
	CannonDistance      = 20.0 // Distance from ship center to cannon (unchanged - visual)
	MaxCannonsPerSide   = 100  // Maximum cannons per side (unchanged)
	MinCannonsPerSide   = 1    // Minimum cannons per side (unchanged)
	CannonSpacingFactor = 0.7  // Factor for spacing cannons along ship length (unchanged)
	ShipScaleFactor     = 1.0  // Base scale factor for ship size (unchanged)
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
	RespawnDelay        = 0.0 // Seconds to wait before respawning
	BaseCollisionDamage = 5   // Base damage dealt per collision
	CollisionCooldown   = 0.2 // Seconds between collision damage ticks
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
