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
	ShipMaxSpeed     = 4.0  // Maximum speed
)

// Message types for client-server communication
const (
	MsgTypeInput    = "input"
	MsgTypeSnapshot = "snapshot"
	MsgTypeJoin     = "join"
	MsgTypeLeave    = "leave"
	MsgTypeScore    = "score"
)

// Player states
const (
	StateAlive = iota
	StateDead
	StateSpawning
)
