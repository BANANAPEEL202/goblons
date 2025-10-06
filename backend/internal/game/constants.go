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
	ShipAcceleration = 1000 // Forward/backward acceleration
	ShipTurnSpeed    = 0.04 // Turning speed in radians per frame
	ShipDeceleration = 0.92 // Drag/friction factor
	ShipMaxSpeed     = 2    // Maximum speed
)

// Cannon and bullet constants
const (
	BulletSpeed         = 2.5  // Bullet travel speed (slower for easier tracking)
	BulletLifetime      = 3.0  // Seconds before bullet disappears (longer for easier spotting)
	BulletSize          = 8.0  // Bullet radius (much larger for visibility)
	CannonCooldown      = 1    // Seconds between shots (faster for testing)
	CannonDistance      = 20.0 // Distance from ship center to cannon
	MaxCannonsPerSide   = 100  // Maximum cannons per side
	MinCannonsPerSide   = 1    // Minimum cannons per side
	CannonSpacingFactor = 0.7  // Factor for spacing cannons along ship length
	ShipScaleFactor     = 1.0  // Base scale factor for ship size
)

// Message types for client-server communication
const (
	MsgTypeInput    = "input"
	MsgTypeSnapshot = "snapshot"
	MsgTypeJoin     = "join"
	MsgTypeLeave    = "leave"
	MsgTypeScore    = "score"
	MsgTypeShoot    = "shoot"
)

// Player states
const (
	StateAlive = iota
	StateDead
	StateSpawning
)
