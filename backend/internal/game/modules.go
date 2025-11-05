package game

import (
	"math"
)

// moduleType defines the category of ship upgrade
type moduleType string

const (
	UpgradeTypeSide  moduleType = "side"  // Cannons on the side of the ship
	UpgradeTypeTop   moduleType = "top"   // Turrets on top of the ship
	UpgradeTypeFront moduleType = "front" // Ram, front cannons, etc.
	UpgradeTypeRear  moduleType = "rear"  // Rudder, rear cannons, etc.
)

// ModuleModifier represents the effects an upgrade has on ship stats
type ModuleModifier struct {
	SpeedMultiplier     float32 `json:"speedMultiplier"`     // Speed modification (1.0 = no change)
	TurnRateMultiplier  float32 `json:"turnRateMultiplier"`  // Turn rate modification (1.0 = no change)
	ShipWidthMultiplier float32 `json:"shipWidthMultiplier"` // Width modification (1.0 = no change)
}

// ShipModule represents a single upgrade installed on a ship
type ShipModule struct {
	ID      uint32         `json:"id"`
	Type    moduleType     `json:"type"`
	Name    string         `json:"name"`
	Count   int            `json:"level"`   // Upgrade level (1, 2, 3, etc.)
	Effect  ModuleModifier `json:"effect"`  // Stat modifications
	Cannons []*Cannon      `json:"cannons"` // Weapons (if applicable)
	Turrets []*Turret      `json:"turrets"` // Turret weapons (if applicable)

	NextUpgrades []*ShipModule `json:"nextUpgrades,omitempty"` // Possible next upgrades
}

// Predefined upgrade templates
func NewBasicSideCannons(cannonCount int) *ShipModule {
	cannonCount = int(math.Max(1, float64(cannonCount))) // Ensure at least 1 cannon per side
	// Create cannons for both sides (cannonCount per side)
	cannons := make([]*Cannon, cannonCount*2)

	// Left side cannons - angle will be calculated dynamically based on ship orientation
	for i := 0; i < cannonCount; i++ {
		cannons[i] = &Cannon{
			ID:    uint32(i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewBasicCannon(),
			Type:  WeaponTypeCannon,
		}
	}

	// Right side cannons - angle will be calculated dynamically based on ship orientation
	for i := 0; i < cannonCount; i++ {
		cannons[cannonCount+i] = &Cannon{
			ID:    uint32(cannonCount + i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewBasicCannon(),
			Type:  WeaponTypeCannon,
		}
	}

	return &ShipModule{
		Type:    UpgradeTypeSide,
		Name:    "Side Cannons",
		Count:   cannonCount,
		Cannons: cannons,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.05, // Slightly slower due to weight
			TurnRateMultiplier:  0,     // avoid double penalty for length and num cannons
			ShipWidthMultiplier: 1.0,
		},
	}
}

func NewScatterSideCannons(cannonCount int) *ShipModule {
	cannonCount = int(math.Max(1, float64(cannonCount))) // Ensure at least 1 cannon per side
	// Create scatter cannons for both sides (cannonCount per side)
	cannons := make([]*Cannon, cannonCount*2)

	// Left side scatter cannons
	for i := 0; i < cannonCount; i++ {
		cannons[i] = &Cannon{
			ID:    uint32(i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewScatterCannon(),
			Type:  WeaponTypeScatter,
		}
	}

	// Right side scatter cannons
	for i := 0; i < cannonCount; i++ {
		cannons[cannonCount+i] = &Cannon{
			ID:    uint32(cannonCount + i + 1),
			Angle: 0, // Relative angle - actual angle calculated during firing
			Stats: NewScatterCannon(),
			Type:  WeaponTypeScatter,
		}
	}

	return &ShipModule{
		Type:    UpgradeTypeSide,
		Name:    "Scatter Cannons",
		Count:   cannonCount,
		Cannons: cannons,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.05, // Slower due to heavier scatter cannons
			TurnRateMultiplier:  -0.05, // Slower turning due to weight and length
			ShipWidthMultiplier: 1,
		},
	}
}

func NewBasicTurrets(turretCount int) *ShipModule {
	turretCount = int(math.Max(0, float64(turretCount))) // Ensure non-negative

	turrets := make([]*Turret, turretCount)
	for i := 0; i < turretCount; i++ {
		turretCannon := Cannon{
			ID:    uint32(i),
			Angle: 0, // Will be controlled by turret aiming
			Stats: NewTurretCannon(),
			Type:  WeaponTypeCannon,
		}
		turret := &Turret{
			ID:      uint32(i + 1),
			Angle:   0, // Will be controlled by turret aiming
			Cannons: []Cannon{turretCannon},
			Type:    WeaponTypeTurret,
		}
		turrets[i] = turret
	}

	return &ShipModule{
		Type:    UpgradeTypeTop,
		Name:    "Basic Turret",
		Count:   turretCount,
		Turrets: turrets,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.03,
			TurnRateMultiplier:  -0.03,
			ShipWidthMultiplier: 1.0,
		},
	}
}

func NewBigTurrets(turretCount int) *ShipModule {
	turretCount = int(math.Max(0, float64(turretCount))) // Ensure non-negative
	turrets := make([]*Turret, turretCount)
	for i := 0; i < turretCount; i++ {
		turretCannon := Cannon{
			ID:    uint32(i),
			Angle: 0, // Will be controlled by turret aiming
			Stats: NewBigCannon(),
			Type:  WeaponTypeCannon,
		}
		turret := &Turret{
			ID:      uint32(i + 1),
			Angle:   0, // Will be controlled by turret aiming
			Cannons: []Cannon{turretCannon},
			Type:    WeaponTypeBigTurret,
		}
		turrets[i] = turret
	}
	return &ShipModule{
		Type:    UpgradeTypeTop,
		Name:    "Big Turret",
		Count:   turretCount,
		Turrets: turrets,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.1,
			TurnRateMultiplier:  -0.1,
			ShipWidthMultiplier: 1.05,
		},
	}
}

func NewMachineGunTurret(turretCount int) *ShipModule {
	turretCount = int(math.Max(0, float64(turretCount))) // Ensure non-negative

	turrets := make([]*Turret, turretCount)
	for i := 0; i < turretCount; i++ {
		// Create two cannons for each machine gu  turret, positioned side by side
		leftCannon := Cannon{
			ID:    uint32(i*2 + 1),
			Angle: 0, // Will be controlled by turret aiming
			Stats: NewMachineGunCannon(),
			Type:  WeaponTypeCannon,
			Position: Position{
				X: 0, // Slightly left of center
				Y: -7,
			},
		}

		rightCannon := Cannon{
			ID:    uint32(i*2 + 2),
			Angle: 0, // Will be controlled by turret aiming
			Stats: NewMachineGunCannon(),
			Type:  WeaponTypeCannon,
			Position: Position{
				X: 0, // Slightly right of center
				Y: 7,
			},
		}

		turret := &Turret{
			ID:              uint32(i + 1),
			Angle:           0, // Will be controlled by turret aiming
			Cannons:         []Cannon{leftCannon, rightCannon},
			Type:            WeaponTypeMachineGunTurret,
			NextCannonIndex: 0, // Start with the first cannon
		}
		turrets[i] = turret
	}

	return &ShipModule{
		Type:    UpgradeTypeTop,
		Name:    "Machine Gun Turret",
		Count:   turretCount,
		Turrets: turrets,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.05, // Slightly more penalty due to heavier turrets
			TurnRateMultiplier:  -0.05,
			ShipWidthMultiplier: 1.1,
		},
	}
}

func NewTopUpgradeTree() *ShipModule {
	root := &ShipModule{
		Type:    UpgradeTypeTop,
		Name:    "No Top Upgrades",
		Turrets: []*Turret{},
	}

	// Build the basic turret upgrade path: 1 -> 2 -> 3 -> 4
	turret1 := NewBasicTurrets(1)
	turret2 := NewBasicTurrets(2)
	turret3 := NewBasicTurrets(3)

	// Build the machine gun turret upgrade path: 1 -> 2
	machineGunTurret1 := NewMachineGunTurret(1)
	machineGunTurret2 := NewMachineGunTurret(2)

	bigTurret1 := NewBigTurrets(1)
	bigTurret2 := NewBigTurrets(2)

	// Link the upgrade paths
	// From root, you can choose basic turret or machine gun turret
	root.NextUpgrades = []*ShipModule{machineGunTurret1, turret1}

	// Basic turret path
	turret1.NextUpgrades = []*ShipModule{bigTurret1, turret2}
	turret2.NextUpgrades = []*ShipModule{turret3}

	bigTurret1.NextUpgrades = []*ShipModule{bigTurret2}

	// machine gun path
	machineGunTurret1.NextUpgrades = []*ShipModule{machineGunTurret2}
	return root
}

func NewSideUpgradeTree() *ShipModule {
	// Build the basic cannon upgrade path: 1 -> 2 -> 3 -> 4
	basic2 := NewBasicSideCannons(2)
	basic3 := NewBasicSideCannons(3)
	basic4 := NewBasicSideCannons(4)

	// Build the scatter cannon branch: 1 (from root)
	scatter1 := NewScatterSideCannons(1)

	// Build the rowing oars branch: 1 -> 2 -> 3
	rowing1 := NewRowingUpgrade(1)
	rowing2 := NewRowingUpgrade(2)
	rowing3 := NewRowingUpgrade(3)

	// Link the basic cannon chain
	basic2.NextUpgrades = []*ShipModule{basic3}
	basic3.NextUpgrades = []*ShipModule{basic4}

	// Link the rowing oars chain
	rowing1.NextUpgrades = []*ShipModule{rowing2}
	rowing2.NextUpgrades = []*ShipModule{rowing3}

	// Root has three paths: upgrade to 2 basic cannons, switch to scatter cannons, or switch to rowing oars
	root := NewBasicSideCannons(1)
	root.NextUpgrades = []*ShipModule{basic2, scatter1, rowing1}

	return root
}

func NewRowingUpgrade(oarCount int) *ShipModule {
	oarCount = int(math.Max(1, float64(oarCount))) // Ensure at least 1 oar per side

	// Create rowing oars as cannons with WeaponTypeRow
	oars := make([]*Cannon, oarCount*2)

	// left side oars
	for i := 0; i < oarCount; i++ {
		oars[i] = &Cannon{
			ID:    uint32(i + 1),
			Angle: 0, // Relative angle - actual angle calculated during rowing
			Stats: NewRowingOar(),
			Type:  WeaponTypeRow,
		}
	}

	// right side oars
	for i := 0; i < oarCount; i++ {
		oars[oarCount+i] = &Cannon{
			ID:    uint32(oarCount + i + 1),
			Angle: 0, // Relative angle - actual angle calculated during rowing
			Stats: NewRowingOar(),
			Type:  WeaponTypeRow,
		}
	}

	return &ShipModule{
		Type:    UpgradeTypeSide,
		Name:    "Rowing Oars",
		Count:   oarCount,
		Cannons: oars,
		Effect: ModuleModifier{
			SpeedMultiplier:     0.05,
			TurnRateMultiplier:  -0.05,
			ShipWidthMultiplier: 1.0, // No effect on width
		},
	}
}

func NewRudderUpgrade() *ShipModule {
	return &ShipModule{
		Type:  UpgradeTypeRear,
		Name:  "Rudder",
		Count: 1,
		Effect: ModuleModifier{
			SpeedMultiplier:     0.0,
			TurnRateMultiplier:  0.2, // Improved turn rate
			ShipWidthMultiplier: 1.0,
		},
	}
}

func NewRamUpgrade() *ShipModule {
	return &ShipModule{
		Type:  UpgradeTypeFront,
		Name:  "Ram",
		Count: 1,
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.3, // Slightly slower due to heavy ram
			TurnRateMultiplier:  -0.3,
			ShipWidthMultiplier: 1.0,
		},
	}
}

func NewChaseCannonUpgrade() *ShipModule {
	cannon1 := &Cannon{
		ID:    1,
		Angle: 0, // Forward facing
		Stats: NewChaseCannon(),
		Type:  WeaponTypeCannon,
	}

	cannon2 := &Cannon{
		ID:    2,
		Angle: 0, // Forward facing
		Stats: NewChaseCannon(),
		Type:  WeaponTypeCannon,
	}

	return &ShipModule{
		Type:  UpgradeTypeFront,
		Name:  "Chase Cannons",
		Count: 2,
		Cannons: []*Cannon{
			cannon1,
			cannon2,
		},
		Effect: ModuleModifier{
			SpeedMultiplier:     -0.05, // Slower due to added weight
			TurnRateMultiplier:  -0.05,
			ShipWidthMultiplier: 1.0,
		},
	}
}

func NewFrontUpgradeTree() *ShipModule {
	root := &ShipModule{
		Type: UpgradeTypeFront,
		Name: "No Front Upgrades",
	}

	ram := NewRamUpgrade()
	chaseCannons := NewChaseCannonUpgrade()
	root.NextUpgrades = []*ShipModule{ram, chaseCannons}

	return root
}

// GetAvailableModules returns the next available upgrades for a given upgrade type
func (sc *ShipConfiguration) GetAvailableModules(upgradeType moduleType) []*ShipModule {
	var availableUpgrades []*ShipModule

	switch upgradeType {
	case UpgradeTypeSide:
		if sc.SideUpgrade == nil {
			// Start with the root of the side upgrade tree
			root := NewSideUpgradeTree()
			return []*ShipModule{root}
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
		if sc.FrontUpgrade == nil || sc.FrontUpgrade.Name == "No Front Upgrades" {
			root := NewFrontUpgradeTree()
			return root.NextUpgrades
		}
		return sc.FrontUpgrade.NextUpgrades

	case UpgradeTypeRear:
		if sc.RearUpgrade == nil {
			root := NewRearUpgradeTree()
			return root.NextUpgrades
		}
		return sc.RearUpgrade.NextUpgrades
	}

	return availableUpgrades
}

func NewRearUpgradeTree() *ShipModule {
	// Placeholder for rear upgrade tree
	root := &ShipModule{
		Type: UpgradeTypeRear,
		Name: "No Rear Upgrades",
	}

	rudder := NewRudderUpgrade()
	root.NextUpgrades = []*ShipModule{rudder}
	return root
}

// ApplyModule applies a selected upgrade to the ship configuration
func (sc *ShipConfiguration) ApplyModule(moduleType moduleType, moduleID string) bool {
	availableModules := sc.GetAvailableModules(moduleType)

	// Find the selected upgrade
	var selectedModule *ShipModule
	for _, module := range availableModules {
		if module.Name == moduleID {
			selectedModule = module
			break
		}
	}

	if selectedModule == nil {
		return false // Upgrade not found
	}

	// Apply the upgrade
	switch moduleType {
	case UpgradeTypeSide:
		sc.SideUpgrade = selectedModule
	case UpgradeTypeTop:
		sc.TopUpgrade = selectedModule
	case UpgradeTypeFront:
		sc.FrontUpgrade = selectedModule
	case UpgradeTypeRear:
		sc.RearUpgrade = selectedModule
	}

	// Recalculate ship dimensions and update positions
	sc.CalculateShipDimensions()
	sc.UpdateUpgradePositions()

	return true
}
