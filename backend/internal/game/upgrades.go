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

	NextUpgrades []*ShipUpgrade `json:"nextUpgrades,omitempty"` // Possible next upgrades

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

			// Right side cannon (negative Y in ship coordinates)w
			sideUpgrade.Cannons[cannonCount+i].Position = Position{
				X: relativeX,
				Y: -sc.ShipWidth/2 - gunWidth/2,
			}
		}
	}

	topUpgrade := sc.TopUpgrade
	if topUpgrade != nil && len(topUpgrade.Turrets) > 0 {
		// Position turrets evenly along the center line of the ship
		// Use consistent spacing with the dimension calculation
		turretSpacing := sc.Size * 0.7

		if len(topUpgrade.Turrets) == 1 {
			// Single turret goes in the center
			topUpgrade.Turrets[0].Position = Position{
				X: 0,
				Y: 0,
			}
			for j := range topUpgrade.Turrets[0].Cannons {
				topUpgrade.Turrets[0].Cannons[j].Position = Position{
					X: 0,
					Y: 0,
				}
			}
		} else {
			// Multiple turrets: space them evenly
			totalTurretLength := turretSpacing * float32(len(topUpgrade.Turrets)-1)
			startOffset := -totalTurretLength / 2

			for i := 0; i < len(topUpgrade.Turrets); i++ {
				offset := startOffset + turretSpacing*float32(i)
				topUpgrade.Turrets[i].Position = Position{
					X: offset,
					Y: 0,
				}
				for j := range topUpgrade.Turrets[i].Cannons {
					topUpgrade.Turrets[i].Cannons[j].Position = Position{
						X: offset,
						Y: 0,
					}
				}
			}
		}
	}
}

// CalculateShipDimensions calculates ship size based on upgrades
func (sc *ShipConfiguration) CalculateShipDimensions() {
	// Start with base dimensions
	baseSize := sc.Size
	length := float32(PlayerSize*1.2) * 0.5 // Base shaft length for 1 cannon
	width := float32(PlayerSize * 0.8)

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
		length = float32(math.Max(float64(length), float64(length+turretLength)))
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

func NewScatterSideCannons(cannonCount int) *ShipUpgrade {
	cannonCount = int(math.Max(1, float64(cannonCount))) // Ensure at least 1 cannon per side
	// Create scatter cannons for both sides (cannonCount per side)
	cannons := make([]Cannon, cannonCount*2)

	// Left side scatter cannons
	for i := 0; i < cannonCount; i++ {
		cannons[i] = Cannon{
			ID:    uint32(i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewScatterCannon(),
			Type:  WeaponTypeScatter,
		}
	}

	// Right side scatter cannons
	for i := 0; i < cannonCount; i++ {
		cannons[cannonCount+i] = Cannon{
			ID:    uint32(cannonCount + i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewScatterCannon(),
			Type:  WeaponTypeScatter,
		}
	}

	return &ShipUpgrade{
		Type:    UpgradeTypeSide,
		Name:    "Scatter Cannons",
		Count:   cannonCount,
		Cannons: cannons,
		Effect: UpgradeEffect{
			SpeedMultiplier:    0.92, // Slower due to heavier scatter cannons
			TurnRateMultiplier: 0.88, // Slower turning due to weight and length
			HealthBonus:        10,   // Slightly more armored
			ArmorBonus:         0.05, // 5% damage reduction
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

func NewTopUpgradeTree() *ShipUpgrade {
	root := &ShipUpgrade{
		Type:    UpgradeTypeTop,
		Name:    "No Top Upgrades",
		Turrets: []Turret{},
	}

	// Build the basic turret upgrade path: 1 -> 2 -> 3 -> 4
	turret1 := NewBasicTurrets(1)
	turret2 := NewBasicTurrets(2)
	turret3 := NewBasicTurrets(3)
	turret4 := NewBasicTurrets(4)

	// Link the chain
	root.NextUpgrades = []*ShipUpgrade{turret1}
	turret1.NextUpgrades = []*ShipUpgrade{turret2}
	turret2.NextUpgrades = []*ShipUpgrade{turret3}
	turret3.NextUpgrades = []*ShipUpgrade{turret4}

	return root
}

func NewSideUpgradeTree() *ShipUpgrade {
	// Build the basic cannon upgrade path: 1 -> 2 -> 3 -> 4
	basic2 := NewBasicSideCannons(2)
	basic3 := NewBasicSideCannons(3)
	basic4 := NewBasicSideCannons(4)

	// Build the scatter cannon branch: 1 (from root)
	scatter1 := NewScatterSideCannons(1)

	// Link the basic cannon chain
	basic2.NextUpgrades = []*ShipUpgrade{basic3}
	basic3.NextUpgrades = []*ShipUpgrade{basic4}

	// Root has two paths: upgrade to 2 basic cannons OR switch to scatter cannons
	root := NewBasicSideCannons(1)
	root.NextUpgrades = []*ShipUpgrade{basic2, scatter1}

	return root
}

// GetAvailableUpgrades returns the next available upgrades for a given upgrade type
func (sc *ShipConfiguration) GetAvailableUpgrades(upgradeType UpgradeType) []*ShipUpgrade {
	var availableUpgrades []*ShipUpgrade

	switch upgradeType {
	case UpgradeTypeSide:
		if sc.SideUpgrade == nil {
			// Start with the root of the side upgrade tree
			root := NewSideUpgradeTree()
			return []*ShipUpgrade{root}
		}
		return sc.SideUpgrade.NextUpgrades

	case UpgradeTypeTop:
		if sc.TopUpgrade == nil || sc.TopUpgrade.Name == "No Top Upgrades" {
			// Start with the root of the top upgrade tree
			root := NewTopUpgradeTree()
			return root.NextUpgrades
		}
		return sc.TopUpgrade.NextUpgrades

	case UpgradeTypeFront:
		if sc.FrontUpgrade == nil {
			// TODO: Implement front upgrade tree when available
			return []*ShipUpgrade{}
		}
		return sc.FrontUpgrade.NextUpgrades

	case UpgradeTypeRear:
		if sc.RearUpgrade == nil {
			// TODO: Implement rear upgrade tree when available
			return []*ShipUpgrade{}
		}
		return sc.RearUpgrade.NextUpgrades
	}

	return availableUpgrades
}

// ApplyUpgrade applies a selected upgrade to the ship configuration
func (sc *ShipConfiguration) ApplyUpgrade(upgradeType UpgradeType, upgradeID string) bool {
	availableUpgrades := sc.GetAvailableUpgrades(upgradeType)

	// Find the selected upgrade
	var selectedUpgrade *ShipUpgrade
	for _, upgrade := range availableUpgrades {
		if upgrade.Name == upgradeID {
			selectedUpgrade = upgrade
			break
		}
	}

	if selectedUpgrade == nil {
		return false // Upgrade not found
	}

	// Apply the upgrade
	switch upgradeType {
	case UpgradeTypeSide:
		sc.SideUpgrade = selectedUpgrade
	case UpgradeTypeTop:
		sc.TopUpgrade = selectedUpgrade
	case UpgradeTypeFront:
		sc.FrontUpgrade = selectedUpgrade
	case UpgradeTypeRear:
		sc.RearUpgrade = selectedUpgrade
	}

	// Recalculate ship dimensions and update positions
	sc.CalculateShipDimensions()
	sc.UpdateUpgradePositions()

	return true
}
