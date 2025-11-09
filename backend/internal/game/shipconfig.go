package game

import (
	"math"
)

// ShipConfiguration holds all upgrades for a ship
type ShipConfiguration struct {
	SideUpgrade  *ShipModule `msgpack:"sideUpgrade"`   // Side cannons upgrade (single)
	TopUpgrade   *ShipModule `msgpack:"topUpgrade"`     // Top turrets upgrade (single)
	FrontUpgrade *ShipModule `msgpack:"frontUpgrade"` // Front weapons upgrade (single)
	RearUpgrade  *ShipModule `msgpack:"rearUpgrade"`   // Rear weapons upgrade (single)
	ShipLength   float64     `msgpack:"shipLength"`     // Calculated ship length based on upgrades
	ShipWidth    float64     `msgpack:"shipWidth"`       // Calculated ship width based on upgrades
	Size         float64     `msgpack:"size"`                 // Base size of the ship
}

// GetTotalEffect calculates the combined effect of all upgrades
func (sc *ShipConfiguration) GetTotalModuleEffects() ModuleModifier {
	effect := ModuleModifier{
		SpeedMultiplier:     1.0,
		TurnRateMultiplier:  1.0,
		ShipWidthMultiplier: 1.0,
	}

	// Collect all non-nil upgrades
	upgrades := []*ShipModule{sc.SideUpgrade, sc.TopUpgrade, sc.FrontUpgrade, sc.RearUpgrade}

	for _, upgrade := range upgrades {
		if upgrade != nil {
			if upgrade.Effect.SpeedMultiplier != 0 {
				effect.SpeedMultiplier *= upgrade.Effect.SpeedMultiplier
			}
			if upgrade.Effect.TurnRateMultiplier != 0 {
				effect.TurnRateMultiplier *= upgrade.Effect.TurnRateMultiplier
			}
			if upgrade.Effect.ShipWidthMultiplier != 0 {
				effect.ShipWidthMultiplier *= upgrade.Effect.ShipWidthMultiplier
			}
		}
	}

	return effect
}

func (sc *ShipConfiguration) GetUpgrade(upgradeType moduleType) *ShipModule {
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
	if sideUpgrade != nil && len(sideUpgrade.Cannons) > 0 {
		// Position side cannons evenly along the sides of the ship
		cannonCount := sideUpgrade.Count // Number of cannons per side
		gunLength := sc.ShipLength * 0.35
		gunWidth := sc.Size * 0.2
		gunSpacing := sc.ShipLength / float64(cannonCount+1)

		for i := 0; i < cannonCount; i++ {
			// Calculate horizontal position along ship length
			cannonLeftEdge := -sc.ShipLength/2 + float64(i+1)*gunSpacing - gunLength/2
			relativeX := cannonLeftEdge + gunLength/2

			// Left side cannon (positive Y in ship coordinates)
			sideUpgrade.Cannons[i].Position = Position{
				X: relativeX,
				Y: sc.ShipWidth/2 + gunWidth/2,
			}
			sideUpgrade.Cannons[i].Angle = float64(math.Pi / 2)

			// Right side cannon (negative Y in ship coordinates)w
			sideUpgrade.Cannons[cannonCount+i].Position = Position{
				X: relativeX,
				Y: -sc.ShipWidth/2 - gunWidth/2,
			}
			sideUpgrade.Cannons[cannonCount+i].Angle = -float64(math.Pi / 2)
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
			totalTurretLength := turretSpacing * float64(len(topUpgrade.Turrets)-1)
			startOffset := -totalTurretLength / 2

			for i := 0; i < len(topUpgrade.Turrets); i++ {
				offset := startOffset + turretSpacing*float64(i)
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

	frontUpgrade := sc.FrontUpgrade
	if frontUpgrade != nil && len(frontUpgrade.Cannons) > 0 {
		// position the 2 front cannons on the left and right sides of the front of the ship
		gunWidth := sc.Size * 0.2
		gunOffsetX := sc.ShipLength/2 + 10
		// left cannon
		frontUpgrade.Cannons[0].Position = Position{
			X: gunOffsetX,
			Y: sc.ShipWidth/2 - gunWidth/2,
		}
		frontUpgrade.Cannons[0].Angle = 0 // Facing forward
		frontUpgrade.Cannons[1].Position = Position{
			X: gunOffsetX,
			Y: -sc.ShipWidth/2 + gunWidth/2,
		}
		frontUpgrade.Cannons[1].Angle = 0 // Facing forward
	}

}

// CalculateShipDimensions calculates ship size based on upgrades
func (sc *ShipConfiguration) CalculateShipDimensions() {
	// Start with base dimensions
	size := sc.Size
	baseLength := float64(size*1.2) * 0.5 // Base shaft length for 1 cannon
	baseWidth := float64(size * 0.8)

	sideLength := baseLength
	turretLength := baseLength

	// Add length for side cannons
	maxSideCannonCount := 0
	if sc.SideUpgrade != nil && len(sc.SideUpgrade.Cannons) > maxSideCannonCount {
		maxSideCannonCount = len(sc.SideUpgrade.Cannons)
	}

	if maxSideCannonCount > 1 {
		gunLength := size * 0.35
		spacing := gunLength * 0.75
		sideLength += spacing * float64(maxSideCannonCount-1)
	}

	// Add length for turrets
	turretCount := 0
	if sc.TopUpgrade != nil {
		turretCount = len(sc.TopUpgrade.Turrets)
	}
	if turretCount > 0 {
		turretSpacing := size * 0.7
		turretLength = baseLength + turretSpacing*float64(turretCount-1)
	}

	sc.ShipLength = max(sideLength, turretLength)
	sc.ShipWidth = max(baseWidth, sc.ShipWidth)
}

// ToMinimalShipConfig converts a ShipConfiguration to MinimalShipConfig for delta snapshots
func (sc *ShipConfiguration) ToMinimalShipConfig() MinimalShipConfig {
	minimal := MinimalShipConfig{
		ShipLength: sc.ShipLength,
		ShipWidth:  sc.ShipWidth,
	}

	// Convert side upgrade
	if sc.SideUpgrade != nil {
		minimal.SideUpgrade = &MinimalShipModule{
			Name:    sc.SideUpgrade.Name,
			Cannons: make([]MinimalCannon, len(sc.SideUpgrade.Cannons)),
		}
		for i, cannon := range sc.SideUpgrade.Cannons {
			minimal.SideUpgrade.Cannons[i] = MinimalCannon{
				Position:   cannon.Position,
				Type:       string(cannon.Type),
				RecoilTime: cannon.RecoilTime,
			}
		}
	}

	// Convert front upgrade
	if sc.FrontUpgrade != nil {
		minimal.FrontUpgrade = &MinimalShipModule{
			Name:    sc.FrontUpgrade.Name,
			Cannons: make([]MinimalCannon, len(sc.FrontUpgrade.Cannons)),
		}
		for i, cannon := range sc.FrontUpgrade.Cannons {
			minimal.FrontUpgrade.Cannons[i] = MinimalCannon{
				Position:   cannon.Position,
				Type:       string(cannon.Type),
				RecoilTime: cannon.RecoilTime,
			}
		}
	}

	// Convert rear upgrade
	if sc.RearUpgrade != nil {
		minimal.RearUpgrade = &MinimalShipModule{
			Name: sc.RearUpgrade.Name,
		}
	}

	// Convert top upgrade (turrets)
	if sc.TopUpgrade != nil {
		minimal.TopUpgrade = &MinimalShipModule{
			Turrets: make([]MinimalTurret, len(sc.TopUpgrade.Turrets)),
		}
		for i, turret := range sc.TopUpgrade.Turrets {
			minimalTurret := MinimalTurret{
				Position:        turret.Position,
				Angle:           turret.Angle,
				Type:            string(turret.Type),
				RecoilTime:      turret.RecoilTime,
				NextCannonIndex: turret.NextCannonIndex,
				Cannons:         make([]MinimalCannon, len(turret.Cannons)),
			}
			for j, cannon := range turret.Cannons {
				minimalTurret.Cannons[j] = MinimalCannon{
					Position:   cannon.Position,
					Type:       string(cannon.Type),
					RecoilTime: cannon.RecoilTime,
				}
			}
			minimal.TopUpgrade.Turrets[i] = minimalTurret
		}
	}

	return minimal
}
