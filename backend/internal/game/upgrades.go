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
	ID      uint32        `json:"id"`
	Type    UpgradeType   `json:"type"`
	Name    string        `json:"name"`
	Count   int           `json:"level"`   // Upgrade level (1, 2, 3, etc.)
	Effect  UpgradeEffect `json:"effect"`  // Stat modifications
	Cannons []Cannon      `json:"cannons"` // Weapons (if applicable)
	Turrets []Turret      `json:"turrets"` // Turret weapons (if applicable)

	ShipWidthModifier  float32 `json:"shipWidthModifier"`  // Width modification (1.0 = no change)
	ShipLengthModifier float32 `json:"shipLengthModifier"` // Length modification (1.0 = no change)
}

// ShipConfiguration holds all upgrades for a ship
type ShipConfiguration struct {
	SideUpgrade  *ShipUpgrade `json:"sideUpgrade"`  // Side cannons upgrade (single)
	TopUpgrade   *ShipUpgrade `json:"topUpgrade"`   // Top turrets upgrade (single)
	FrontUpgrade *ShipUpgrade `json:"frontUpgrade"` // Front weapons upgrade (single)
	RearUpgrade  *ShipUpgrade `json:"rearUpgrade"`  // Rear weapons upgrade (single)
	ShipLength   float32      `json:"shipLength"`   // Calculated ship length based on upgrades
	ShipWidth    float32      `json:"shipWidth"`    // Calculated ship width based on upgrades
	Size         float32      `json:"size"`         // Base size of the ship
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

func (sc *ShipConfiguration) GetUpgrade(upgradeType UpgradeType) *ShipUpgrade {
	switch upgradeType {
	case UpgradeTypeSide:
		return sc.SideUpgrade
	case UpgradeTypeTop:
		return sc.TopUpgrade
	case UpgradeTypeFront:
		return sc.FrontUpgrade
	case UpgradeTypeRear:
		return sc.RearUpgrade
	default:
		return nil
	}
}

func (sc *ShipConfiguration) UpdateUpgradePositions() {
	sideUpgrade := sc.SideUpgrade
	if sideUpgrade != nil {
		// Position side cannons evenly along the sides of the ship
		cannonCount := sideUpgrade.Count // Number of cannons per side
		gunLength := sc.ShipLength * 0.35
		gunWidth := sc.Size * 0.2
		gunSpacing := sc.ShipLength / float32(cannonCount+1)

		for i := 0; i < cannonCount; i++ {
			// Calculate horizontal position along ship length
			cannonLeftEdge := -sc.ShipLength/2 + float32(i+1)*gunSpacing - gunLength/2
			relativeX := cannonLeftEdge + gunLength/2

			// Left side cannon (positive Y in ship coordinates)
			sideUpgrade.Cannons[i].Position = Position{
				X: relativeX,
				Y: sc.ShipWidth/2 + gunWidth/2,
			}

			// Right side cannon (negative Y in ship coordinates)
			sideUpgrade.Cannons[cannonCount+i].Position = Position{
				X: relativeX,
				Y: -sc.ShipWidth/2 - gunWidth/2,
			}
		}
	}

	topUpgrade := sc.TopUpgrade
	if topUpgrade != nil {
		// Position turrets evenly along the center line of the ship
		turretSpacing := float64(sc.ShipLength) / float64(topUpgrade.Count+1)

		for i := 0; i < topUpgrade.Count; i++ {
			offset := -float64(sc.ShipLength/2) + turretSpacing*float64(i+1)
			topUpgrade.Turrets[i].Position = Position{
				X: float32(offset),
				Y: 0,
			}
			for j := range topUpgrade.Turrets[i].Cannons {
				topUpgrade.Turrets[i].Cannons[j].Position = Position{
					X: float32(offset),
					Y: 0,
				}
			}
		}
	}
}

// CalculateShipDimensions calculates ship size based on upgrades
func (sc *ShipConfiguration) CalculateShipDimensions() {
	// Start with base dimensions
	baseSize := sc.Size
	length := baseSize * 0.5
	width := baseSize * 0.8

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

	sc.ShipLength = length
	sc.ShipWidth = width
}

// Predefined upgrade templates
func NewBasicSideCannons(cannonCount int) *ShipUpgrade {
	cannonCount = int(math.Max(1, float64(cannonCount))) // Ensure at least 1 cannon per side
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

	return &ShipUpgrade{
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

func NewBasicTurrets(turretCount int) *ShipUpgrade {
	turretCount = int(math.Max(0, float64(turretCount))) // Ensure non-negative
	turretCannon := Cannon{
		ID:    1,
		Angle: 0, // Will be controlled by turret aiming
		Stats: NewTurretCannon(),
		Type:  WeaponTypeCannon,
	}

	turrets := make([]Turret, turretCount)
	for i := 0; i < turretCount; i++ {
		turret := Turret{
			ID:      uint32(i + 1),
			Angle:   0, // Will be controlled by turret aiming
			Cannons: []Cannon{turretCannon},
			Type:    WeaponTypeTurret,
		}
		turrets[i] = turret
	}

	return &ShipUpgrade{
		Type:    UpgradeTypeTop,
		Name:    "Basic Turret",
		Count:   turretCount,
		Turrets: turrets,
		Effect: UpgradeEffect{
			SpeedMultiplier:    0.98,
			TurnRateMultiplier: 0.95,
			HealthBonus:        0,
			ArmorBonus:         0,
		},
	}
}
