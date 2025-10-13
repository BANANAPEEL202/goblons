package game

import (
	"math"
)

// UpgradeType defines the category of ship upgrade
type UpgradeType string

const (
	UpgradeTypeSide  UpgradeType = "side"  // Cannons on the side of the ship
	UpgradeTypeTop   UpgradeType = "top"   // Turrets on top of the ship
	UpgradeTypeFront UpgradeType = "front" // Ram, front cannons, etc.
	UpgradeTypeRear  UpgradeType = "rear"  // Rudder, rear cannons, etc.
)

// UpgradeEffect represents the effects an upgrade has on ship stats
type UpgradeEffect struct {
	SpeedMultiplier    float32 `json:"speedMultiplier"`    // Speed modification (1.0 = no change)
	TurnRateMultiplier float32 `json:"turnRateMultiplier"` // Turn rate modification (1.0 = no change)
	HealthBonus        int     `json:"healthBonus"`        // Additional health points
	ArmorBonus         float32 `json:"armorBonus"`         // Damage reduction (0.1 = 10% reduction)
}

// ShipUpgrade represents a single upgrade installed on a ship
type ShipUpgrade struct {
	ID        uint32        `json:"id"`
	Type      UpgradeType   `json:"type"`
	Name      string        `json:"name"`
	Count     int           `json:"level"`     // Upgrade level (1, 2, 3, etc.)
	Effect    UpgradeEffect `json:"effect"`    // Stat modifications
	Cannons   []Cannon      `json:"cannons"`   // Weapons (if applicable)
	Turrets   []Turret      `json:"turrets"`   // Turret weapons (if applicable)
	Positions []Position    `json:"positions"` // Positions for multiple cannons/turrets

	ShipWidthModifier  float32 `json:"shipWidthModifier"`  // Width modification (1.0 = no change)
	ShipLengthModifier float32 `json:"shipLengthModifier"` // Length modification (1.0 = no change)
}

// ShipConfiguration holds all upgrades for a ship
type ShipConfiguration struct {
	SideUpgrade  *ShipUpgrade `json:"sideUpgrade"`  // Side cannons upgrade (single)
	TopUpgrade   *ShipUpgrade `json:"topUpgrade"`   // Top turrets upgrade (single)
	FrontUpgrade *ShipUpgrade `json:"frontUpgrade"` // Front weapons upgrade (single)
	RearUpgrade  *ShipUpgrade `json:"rearUpgrade"`  // Rear weapons upgrade (single)
}

// GetTotalEffect calculates the combined effect of all upgrades
func (sc *ShipConfiguration) GetTotalEffect() UpgradeEffect {
	effect := UpgradeEffect{
		SpeedMultiplier:    1.0,
		TurnRateMultiplier: 1.0,
		HealthBonus:        0,
		ArmorBonus:         0,
	}

	// Collect all non-nil upgrades
	upgrades := []*ShipUpgrade{sc.SideUpgrade, sc.TopUpgrade, sc.FrontUpgrade, sc.RearUpgrade}

	for _, upgrade := range upgrades {
		if upgrade != nil {
			effect.SpeedMultiplier *= upgrade.Effect.SpeedMultiplier
			effect.TurnRateMultiplier *= upgrade.Effect.TurnRateMultiplier
			effect.HealthBonus += upgrade.Effect.HealthBonus
			effect.ArmorBonus += upgrade.Effect.ArmorBonus
		}
	}

	return effect
}

func (sc *ShipUpgrade) UpdateUpgradePositions(player *Player) {
	if sc.Type == UpgradeTypeSide && len(sc.Cannons) > 0 {
		// Position side cannons evenly along the sides of the ship
		cannonCount := sc.Count // Number of cannons per side
		gunLength := player.ShipLength * 0.35
		gunWidth := player.Size * 0.2
		gunSpacing := player.ShipLength / float32(cannonCount+1)

		for i := 0; i < cannonCount; i++ {
			// Calculate horizontal position along ship length
			cannonLeftEdge := -player.ShipLength/2 + float32(i+1)*gunSpacing - gunLength/2
			relativeX := cannonLeftEdge + gunLength/2

			// Left side cannon (positive Y in ship coordinates)
			sc.Cannons[i].Position = Position{
				X: relativeX,
				Y: player.ShipWidth/2 + gunWidth/2,
			}

			// Right side cannon (negative Y in ship coordinates)
			sc.Cannons[cannonCount+i].Position = Position{
				X: relativeX,
				Y: -player.ShipWidth/2 - gunWidth/2,
			}
		}
	} else if sc.Type == UpgradeTypeTop && len(sc.Turrets) > 0 {
		// Position turrets evenly along the center line of the ship
		turretSpacing := ShipScaleFactor * 0.7
		totalLength := turretSpacing * float64(len(sc.Turrets)-1)

		for i := 0; i < len(sc.Turrets); i++ {
			offset := -totalLength/2 + turretSpacing*float64(i)
			sc.Turrets[i].Position = Position{
				X: float32(offset),
				Y: 0,
			}
		}
	}

}

// CalculateShipDimensions calculates ship size based on upgrades
func (sc *ShipConfiguration) CalculateShipDimensions(baseSize float32) (length, width float32) {
	// Start with base dimensions
	length = baseSize * 0.5
	width = baseSize * 0.8

	// Add length for side cannons
	maxSideCannonCount := 0
	if sc.SideUpgrade != nil && len(sc.SideUpgrade.Cannons) > maxSideCannonCount {
		maxSideCannonCount = len(sc.SideUpgrade.Cannons)
	}

	if maxSideCannonCount > 1 {
		gunLength := baseSize * 0.35
		spacing := gunLength * 0.75
		length += spacing * float32(maxSideCannonCount-1)
	}

	// Add length for turrets
	turretCount := 0
	if sc.TopUpgrade != nil {
		turretCount = len(sc.TopUpgrade.Turrets)
	}
	if turretCount > 0 {
		turretSpacing := baseSize * 0.7
		turretLength := turretSpacing * float32(turretCount-1)
		length = float32(math.Max(float64(length), float64(baseSize*1.2+turretLength)))
		width *= 1.1 // Slightly wider for turrets
	}

	return length, width
}

// Predefined upgrade templates
func NewBasicSideCannons(unused bool, cannonCount int) ShipUpgrade {
	// Create cannons for both sides (cannonCount per side)
	cannons := make([]Cannon, cannonCount*2)

	// Left side cannons - angle will be calculated dynamically based on ship orientation
	for i := 0; i < cannonCount; i++ {
		cannons[i] = Cannon{
			ID:    uint32(i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewBasicCannon(),
			Type:  WeaponTypeCannon,
		}
	}

	// Right side cannons - angle will be calculated dynamically based on ship orientation
	for i := 0; i < cannonCount; i++ {
		cannons[cannonCount+i] = Cannon{
			ID:    uint32(cannonCount + i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewBasicCannon(),
			Type:  WeaponTypeCannon,
		}
	}

	return ShipUpgrade{
		Type:    UpgradeTypeSide,
		Name:    "Side Cannons",
		Count:   cannonCount,
		Cannons: cannons,
		Effect: UpgradeEffect{
			SpeedMultiplier:    0.95, // Slightly slower due to weight
			TurnRateMultiplier: 0.9,  // Slower turning due to length
			HealthBonus:        0,
			ArmorBonus:         0,
		},
	}
}

func NewBasicTurret(position struct{ X, Y float32 }) ShipUpgrade {
	turretCannon := Cannon{
		ID:    1,
		Angle: 0, // Will be controlled by turret aiming
		Stats: NewTurretCannon(),
		Type:  WeaponTypeCannon,
	}

	turret := Turret{
		ID:      1,
		Angle:   0,
		Cannons: []Cannon{turretCannon},
		Type:    WeaponTypeTurret,
	}

	return ShipUpgrade{
		Type:    UpgradeTypeTop,
		Name:    "Basic Turret",
		Count:   1,
		Turrets: []Turret{turret},
		Effect: UpgradeEffect{
			SpeedMultiplier:    0.98,
			TurnRateMultiplier: 0.95,
			HealthBonus:        10,
			ArmorBonus:         0,
		},
	}
}
