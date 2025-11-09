package game

import (
	"log"
	"math/rand"
	"time"
)

type Mods struct {
	SpeedMultiplier        float32
	HealthRegenPerSec      float32
	BulletSpeedMultiplier  float32
	BulletDamageMultiplier float32
	ReloadSpeedMultiplier  float32
	MoveSpeedMultiplier    float32
	TurnSpeedMultiplier    float32
	BodyDamageBonus        float32
}

// spawn spawns a player at a random safe location
func (player *Player) spawn() {
	// Simple random spawn - could be improved to avoid other players
	player.X = float32(rand.Intn(int(WorldWidth-100)) + 50)
	player.Y = float32(rand.Intn(int(WorldHeight-100)) + 50)
	player.State = StateAlive
	player.SpawnTime = time.Now() // Track when player spawned
}

// respawnPlayer respawns a dead player when they request it
func (player *Player) respawn() {
	now := time.Now()

	// Only respawn if player is dead and respawn time has passed
	if player.State != StateDead || now.Before(player.RespawnTime) {
		return
	}

	// Save half of previous XP and coins
	respawnXP := player.Experience / 2
	respawnCoins := player.Coins / 2

	// Save player identity
	playerID := player.ID
	playerName := player.Name
	playerColor := player.Color

	// Reset to fresh player state (similar to NewPlayer)
	player.Experience = respawnXP
	player.Coins = respawnCoins
	player.Level = 1
	player.AvailableUpgrades = 0
	player.Score = 0
	player.Health = 100
	player.MaxHealth = 100
	player.State = StateAlive
	player.LastRegenTime = now
	player.LastCollisionDamage = now

	// Restore identity
	player.ID = playerID
	player.Name = playerName
	player.Color = playerColor

	// Reset death tracking
	player.KilledBy = 0
	player.KilledByName = ""
	player.ScoreAtDeath = 0
	player.SurvivalTime = 0

	// Reset autofire to default enabled state
	player.AutofireEnabled = false

	player.resetPlayerShipConfig()

	player.Modifiers = Mods{
		SpeedMultiplier:        1.0,
		HealthRegenPerSec:      1.0,
		BulletSpeedMultiplier:  1.0,
		BulletDamageMultiplier: 1.0,
		ReloadSpeedMultiplier:  1.0,
		MoveSpeedMultiplier:    1.0,
		TurnSpeedMultiplier:    1.0,
		BodyDamageBonus:        1.0,
	}

	// Reset stat upgrades
	InitializeStatUpgrades(player)

	player.spawn()

	// Send updated available upgrades to client
	sendAvailableUpgrades(player.Client)

	log.Printf("Player %d (%s) respawned with %d XP and %d coins", player.ID, player.Name, respawnXP, respawnCoins)
}

// updateShipGeometry updates ship dimensions based on cannon and turret count
func (player *Player) updateShipGeometry() {
	sc := &player.ShipConfig
	sc.CalculateShipDimensions()

	// Update positions for all upgrades
	sc.UpdateUpgradePositions()
}

// resetPlayerShipConfig resets a player's ship configuration to default
func (player *Player) resetPlayerShipConfig() {
	// Reset ship configuration to basic setup
	shipLength := float32(PlayerSize) * 1.2
	shipWidth := float32(PlayerSize) * 0.6

	player.ShipConfig = ShipConfiguration{

		SideUpgrade:  NewSideUpgradeTree(),
		TopUpgrade:   NewTopUpgradeTree(),
		FrontUpgrade: NewFrontUpgradeTree(),
		RearUpgrade:  NewRearUpgradeTree(),
		ShipLength:   shipLength,
		ShipWidth:    shipWidth,
		Size:         PlayerSize,
	}

	// Recalculate ship dimensions and positions
	player.updateShipGeometry()
}

// copyPlayer creates a deep copy of a Player including maps
func copyPlayer(player Player) Player {
	copy := player

	// Deep copy the Upgrades map
	if player.Upgrades != nil {
		copy.Upgrades = make(map[UpgradeType]Upgrade)
		for k, v := range player.Upgrades {
			copy.Upgrades[k] = v
		}
	}

	// Deep copy the ActionCooldowns map
	if player.ActionCooldowns != nil {
		copy.ActionCooldowns = make(map[string]time.Time)
		for k, v := range player.ActionCooldowns {
			copy.ActionCooldowns[k] = v
		}
	}

	return copy
}

// hasPlayerChanges checks if a delta player has any changed fields
func hasPlayerChanges(delta DeltaPlayer) bool {
	return delta.X != nil ||
		delta.Y != nil ||
		delta.VelX != nil ||
		delta.VelY != nil ||
		delta.Angle != nil ||
		delta.Score != nil ||
		delta.State != nil ||
		delta.Name != nil ||
		delta.Color != nil ||
		delta.Health != nil ||
		delta.MaxHealth != nil ||
		delta.Level != nil ||
		delta.Experience != nil ||
		delta.AvailableUpgrades != nil ||
		delta.Coins != nil ||
		delta.Upgrades != nil ||
		delta.AutofireEnabled != nil ||
		delta.DebugInfo != nil
}

// BuyUpgrade attempts to upgrade a specific stat for a player
func (player *Player) BuyUpgrade(upgradeType UpgradeType) bool {
	if player.Upgrades == nil {
		InitializeStatUpgrades(player)
	}

	upgrade, exists := player.Upgrades[upgradeType]
	if !exists {
		return false
	}

	// Check if upgrade is maxed out
	if upgrade.Level >= upgrade.MaxLevel {
		return false
	}

	// Calculate total upgrades across all stats
	totalUpgrades := 0
	for _, statUpgrade := range player.Upgrades {
		totalUpgrades += statUpgrade.Level
	}

	// Check if total upgrade limit is reached (75)
	if totalUpgrades >= 75 {
		return false
	}

	// Check if player has enough coins
	if player.Coins < upgrade.CurrentCost {
		return false
	}

	// Deduct coins and upgrade
	player.Coins -= upgrade.CurrentCost
	upgrade.Level++
	upgrade.CurrentCost = upgrade.BaseCost * (upgrade.Level + 1) // 10, 20, 30, etc.
	player.Upgrades[upgradeType] = upgrade

	// Apply upgrade effects to player
	player.updateModifiers()

	if upgradeType == StatUpgradeHullStrength {
		player.Health = min(player.Health+HealthIncrease, player.MaxHealth)
		player.ShipConfig.ShipWidth *= 1.01 // Small width increase per level
		player.ShipConfig.UpdateUpgradePositions()
	}

	return true
}

// updateModifiers applies the effects of a stat upgrade to the player
// modifiers are percentage multipliers off base values
// stack additively
func (player *Player) updateModifiers() {
	sc := &player.ShipConfig
	moduleSpeedModifier := float32(0)
	moduleTurnSpeedMultiplier := float32(0)
	modules := []*ShipModule{sc.SideUpgrade, sc.TopUpgrade, sc.FrontUpgrade, sc.RearUpgrade}

	for _, module := range modules {
		if module != nil {
			moduleSpeedModifier += module.Effect.SpeedMultiplier * float32(module.Count)
			moduleTurnSpeedMultiplier += module.Effect.TurnRateMultiplier * float32(module.Count)

		}
	}

	healthLevel := player.Upgrades[StatUpgradeHullStrength].Level
	player.MaxHealth = 100 + (healthLevel * HealthIncrease)

	hullLevel := player.Upgrades[StatUpgradeHullStrength].Level
	moveLevel := player.Upgrades[StatUpgradeMoveSpeed].Level
	ramLevel := player.Upgrades[StatUpgradeBodyDamage].Level
	// speed multipler is -1% per hull level, +2% per move level
	player.Modifiers.MoveSpeedMultiplier = 1.0 - float32(hullLevel)*0.01 - float32(ramLevel)*0.01 + float32(moveLevel)*0.02
	player.Modifiers.MoveSpeedMultiplier += moduleSpeedModifier

	repairLevel := player.Upgrades[StatUpgradeAutoRepairs].Level
	player.Modifiers.HealthRegenPerSec = float32(repairLevel) * 0.6

	rangeLevel := player.Upgrades[StatUpgradeCannonRange].Level
	player.Modifiers.BulletSpeedMultiplier = 1.0 + (float32(rangeLevel) * 0.05)

	damageLevel := player.Upgrades[StatUpgradeCannonDamage].Level
	player.Modifiers.BulletDamageMultiplier = 1.0 + (float32(damageLevel) * 0.08)

	reloadLevel := player.Upgrades[StatUpgradeReloadSpeed].Level
	player.Modifiers.ReloadSpeedMultiplier = 1.0 - (float32(reloadLevel) * 0.03) // 2% faster per level

	turnLevel := player.Upgrades[StatUpgradeTurnSpeed].Level
	player.Modifiers.TurnSpeedMultiplier = 1 + float32(turnLevel)*0.02 - float32(ramLevel)*0.01
	player.Modifiers.TurnSpeedMultiplier += moduleTurnSpeedMultiplier

	player.Modifiers.BodyDamageBonus = float32(ramLevel) * 0.5
}
