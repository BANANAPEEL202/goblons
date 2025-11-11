package game

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/vmihailenco/msgpack/v5"
)

// calculateItemDeltas compares current items with client's last snapshot to find added/removed items
func (w *World) calculateItemDeltas(currentItems []GameItem, lastSnapshot Snapshot) ([]GameItem, []uint32) {
	// Create maps for efficient lookup
	lastItemMap := make(map[uint32]GameItem)
	for _, item := range lastSnapshot.Items {
		lastItemMap[item.ID] = item
	}

	currentItemMap := make(map[uint32]GameItem)
	for _, item := range currentItems {
		currentItemMap[item.ID] = item
	}

	var itemsAdded []GameItem
	var itemsRemoved []uint32

	// Find added items (in current but not in last)
	for _, item := range currentItems {
		if _, exists := lastItemMap[item.ID]; !exists {
			itemsAdded = append(itemsAdded, item)
		}
	}

	// Find removed items (in last but not in current)
	for _, item := range lastSnapshot.Items {
		if _, exists := currentItemMap[item.ID]; !exists {
			itemsRemoved = append(itemsRemoved, item.ID)
		}
	}

	return itemsAdded, itemsRemoved
}

// calculateBulletDeltas compares current bullets with client's last snapshot to find added/removed bullets
func (w *World) calculateBulletDeltas(currentBullets []Bullet, lastSnapshot Snapshot) ([]Bullet, []uint32) {
	// Create maps for efficient lookup
	lastBulletMap := make(map[uint32]Bullet)
	for _, bullet := range lastSnapshot.Bullets {
		lastBulletMap[bullet.ID] = bullet
	}

	currentBulletMap := make(map[uint32]Bullet)
	for _, bullet := range currentBullets {
		currentBulletMap[bullet.ID] = bullet
	}

	var bulletsAdded []Bullet
	var bulletsRemoved []uint32

	// Find added bullets (in current but not in last)
	for _, bullet := range currentBullets {
		if _, exists := lastBulletMap[bullet.ID]; !exists {
			bulletsAdded = append(bulletsAdded, bullet)
		}
	}

	// Find removed bullets (in last but not in current)
	for _, bullet := range lastSnapshot.Bullets {
		if _, exists := currentBulletMap[bullet.ID]; !exists {
			bulletsRemoved = append(bulletsRemoved, bullet.ID)
		}
	}

	return bulletsAdded, bulletsRemoved
}

// GetSnapshotStats returns the current snapshot statistics
func (w *World) GetSnapshotStats() (count int64, totalSize int64) {
	return atomic.LoadInt64(&w.snapshotCount), atomic.LoadInt64(&w.totalSnapshotSize)
}

// getBulletsInRange returns bullets within visible range of a player
func (w *World) getBulletsInRange(player *Player) []Bullet {
	bullets := make([]Bullet, 0, 50) // Pre-allocate reasonable capacity
	maxBullets := 200                // Limit bullets per client to prevent overload

	bulletCount := 0
	for _, bullet := range w.bullets {
		if bulletCount >= maxBullets {
			break
		}

		// Calculate distance squared (avoid sqrt for performance)
		dx := bullet.X - player.X
		dy := bullet.Y - player.Y
		distSq := dx*dx + dy*dy

		// Include bullet if within visible range
		if distSq <= BulletVisibleRange*BulletVisibleRange {
			bullets = append(bullets, *bullet)
			bulletCount++
		}
	}

	return bullets
}

// broadcastSnapshot sends the current game state to all clients (optimized)
func (w *World) broadcastSnapshot() {
	// Limit data to reduce bandwidth
	maxItems := MaxItems * 2

	currentSnapshot := Snapshot{
		Type:    MsgTypeSnapshot,
		Players: make([]Player, 0, len(w.players)),
		Items:   make([]GameItem, 0, min(len(w.items), maxItems)),
		Bullets: []Bullet{},
		Time:    time.Now().UnixMilli(),
	}

	// Add all players to snapshot
	for _, player := range w.players {
		// Calculate debug info for this player
		player.DebugInfo = w.calculateDebugInfo(player)
		currentSnapshot.Players = append(currentSnapshot.Players, copyPlayer(*player))
	}

	// Add limited items to snapshot (prioritize closer items for performance)
	itemCount := 0
	for _, item := range w.items {
		if itemCount >= maxItems {
			break
		}
		currentSnapshot.Items = append(currentSnapshot.Items, *item)
		itemCount++
	}

	// Send to all clients concurrently (non-blocking)
	for _, client := range w.clients {
		go func(c *Client) {
			var data []byte
			var err error

			c.mu.RLock()
			isFirstSnapshot := c.lastSnapshot.Time == 0
			c.mu.RUnlock()

			// Create client-specific snapshot with filtered bullets
			clientSnapshot := currentSnapshot
			clientSnapshot.Bullets = w.getBulletsInRange(c.Player)

			if isFirstSnapshot {
				// First snapshot for this client - send full snapshot
				data, err = msgpack.Marshal(clientSnapshot)
				if err != nil {
					log.Printf("Error marshaling snapshot for client %d: %v", c.ID, err)
					return
				}
			} else {
				// Calculate delta changes for items based on client's last snapshot
				c.mu.RLock()
				itemsAdded, itemsRemoved := w.calculateItemDeltas(clientSnapshot.Items, c.lastSnapshot)
				bulletsAdded, bulletsRemoved := w.calculateBulletDeltas(clientSnapshot.Bullets, c.lastSnapshot)
				c.mu.RUnlock()

				// Calculate player deltas based on client's last snapshot
				var playerDeltas []PlayerDelta
				lastPlayerMap := make(map[uint32]*Player)
				for i := range c.lastSnapshot.Players {
					lastPlayerMap[c.lastSnapshot.Players[i].ID] = &c.lastSnapshot.Players[i]
				}

				for _, currentPlayer := range clientSnapshot.Players {
					if lastPlayer, exists := lastPlayerMap[currentPlayer.ID]; exists {
						delta := calculatePlayerDeltas(lastPlayer, &currentPlayer)
						// Only include deltas that have changes (at least one field changed)
						if hasPlayerChanges(delta) {
							playerDeltas = append(playerDeltas, delta)
						}
					} else {
						// New player - send all fields
						delta := PlayerDelta{
							ID:                currentPlayer.ID,
							X:                 &currentPlayer.X,
							Y:                 &currentPlayer.Y,
							VelX:              &currentPlayer.VelX,
							VelY:              &currentPlayer.VelY,
							Angle:             &currentPlayer.Angle,
							Score:             &currentPlayer.Score,
							State:             &currentPlayer.State,
							Name:              &currentPlayer.Name,
							Color:             &currentPlayer.Color,
							Health:            &currentPlayer.Health,
							MaxHealth:         &currentPlayer.MaxHealth,
							Level:             &currentPlayer.Level,
							Experience:        &currentPlayer.Experience,
							AvailableUpgrades: &currentPlayer.AvailableUpgrades,
							ShipConfig:        currentPlayer.ShipConfig.ToMinimalShipConfig(),
							Coins:             &currentPlayer.Coins,
							Upgrades:          &currentPlayer.Upgrades,
							AutofireEnabled:   &currentPlayer.AutofireEnabled,
							DebugInfo:         &currentPlayer.DebugInfo,
						}
						playerDeltas = append(playerDeltas, delta)
					}
				}

				// Create delta snapshot
				deltaSnapshot := DeltaSnapshot{
					Type:           MsgTypeDeltaSnapshot,
					Players:        playerDeltas,
					ItemsAdded:     itemsAdded,
					ItemsRemoved:   itemsRemoved,
					BulletsAdded:   bulletsAdded,
					BulletsRemoved: bulletsRemoved,
					Time:           clientSnapshot.Time,
				}

				data, err = msgpack.Marshal(deltaSnapshot)
				if err != nil {
					log.Printf("Error marshaling delta snapshot for client %d: %v", c.ID, err)
					return
				}
			}

			// Store current snapshot for this client's next delta calculation
			c.mu.Lock()
			c.lastSnapshot = clientSnapshot
			c.mu.Unlock()

			// Send to client
			select {
			case c.Send <- data:
				// Track snapshot size
				atomic.AddInt64(&w.snapshotCount, 1)
				atomic.AddInt64(&w.totalSnapshotSize, int64(len(data)))
			case <-time.After(10 * time.Millisecond):
				// Skip slow clients to prevent blocking
			}
		}(client)
	}
}

func calculateShipConfigDeltas(oldConfig, newConfig *ShipConfiguration) ShipConfigDelta {
	delta := ShipConfigDelta{}

	if oldConfig.ShipLength != newConfig.ShipLength {
		delta.ShipLength = newConfig.ShipLength
	}
	if oldConfig.ShipWidth != newConfig.ShipWidth {
		delta.ShipWidth = newConfig.ShipWidth
	}

	// Compare side upgrade
	delta.SideUpgrade = calculateShipModuleDelta(oldConfig.SideUpgrade, newConfig.SideUpgrade)

	// Compare front upgrade
	delta.FrontUpgrade = calculateShipModuleDelta(oldConfig.FrontUpgrade, newConfig.FrontUpgrade)

	// Compare rear upgrade
	delta.RearUpgrade = calculateShipModuleDelta(oldConfig.RearUpgrade, newConfig.RearUpgrade)

	// Compare top upgrade (turrets)
	delta.TopUpgrade = calculateShipModuleDelta(oldConfig.TopUpgrade, newConfig.TopUpgrade)

	return delta
}

func calculateShipModuleDelta(oldModule, newModule *ShipModule) *ShipModuleDelta {
	if oldModule == nil && newModule == nil {
		return nil
	}
	delta := &ShipModuleDelta{}

	if oldModule == nil || newModule == nil || oldModule.Name != newModule.Name {
		delta.Name = newModule.Name
	}

	// Compare cannons
	delta.Cannons = calculateCannonDeltas(oldModule.Cannons, newModule.Cannons)

	// compare turrets
	delta.Turrets = calculateTurretDeltas(newModule.Turrets)

	// Return nil if no changes were detected
	if delta.Name == "" && len(delta.Cannons) == 0 && len(delta.Turrets) == 0 {
		return nil
	}

	return delta
}

func calculateTurretDeltas(newTurrets []*Turret) []TurretDelta {
	delta := []TurretDelta{}
	for _, turret := range newTurrets {
		// Convert []Cannon to []*Cannon
		var cannonPtrs []*Cannon
		for i := range turret.Cannons {
			cannonPtrs = append(cannonPtrs, &turret.Cannons[i])
		}
		turretDelta := TurretDelta{
			Position:        turret.Position,
			Angle:           turret.Angle,
			Type:            string(turret.Type),
			NextCannonIndex: turret.NextCannonIndex,
			Cannons:         calculateCannonDeltas(nil, cannonPtrs),
		}
		delta = append(delta, turretDelta)
	}
	return delta
}

func calculateCannonDeltas(oldCannons, newCannons []*Cannon) []CannonDelta {
	if len(oldCannons) != len(newCannons) {
		deltas := make([]CannonDelta, len(newCannons))
		for i, cannon := range newCannons {
			deltas[i] = CannonDelta{
				Position:   cannon.Position,
				Type:       string(cannon.Type),
				RecoilTime: cannon.RecoilTime,
			}
		}
		return deltas
	}

	var deltas []CannonDelta
	for i := range newCannons {
		oldCannon := oldCannons[i]
		newCannon := newCannons[i]
		if oldCannon.Position != newCannon.Position || oldCannon.Type != newCannon.Type || !newCannon.RecoilTime.IsZero() {
			delta := CannonDelta{
				Position:   newCannon.Position,
				Type:       string(newCannon.Type),
				RecoilTime: newCannon.RecoilTime,
			}
			deltas = append(deltas, delta)
		}
	}
	return deltas
}

// calculatePlayerDeltas compares two players and returns only the changed fields
func calculatePlayerDeltas(oldPlayer, newPlayer *Player) PlayerDelta {
	delta := PlayerDelta{
		ID: newPlayer.ID, // Always include ID
	}

	// Compare position and movement (changes frequently)
	if oldPlayer.X != newPlayer.X {
		delta.X = &newPlayer.X
	}
	if oldPlayer.Y != newPlayer.Y {
		delta.Y = &newPlayer.Y
	}
	if oldPlayer.VelX != newPlayer.VelX {
		delta.VelX = &newPlayer.VelX
	}
	if oldPlayer.VelY != newPlayer.VelY {
		delta.VelY = &newPlayer.VelY
	}
	if oldPlayer.Angle != newPlayer.Angle {
		delta.Angle = &newPlayer.Angle
	}

	// Compare state and score (changes occasionally)
	if oldPlayer.Score != newPlayer.Score {
		delta.Score = &newPlayer.Score
	}
	if oldPlayer.State != newPlayer.State {
		delta.State = &newPlayer.State
	}

	// Compare name and color (changes rarely)
	if oldPlayer.Name != newPlayer.Name {
		delta.Name = &newPlayer.Name
	}
	if oldPlayer.Color != newPlayer.Color {
		delta.Color = &newPlayer.Color
	}

	// Compare health (changes frequently)
	if oldPlayer.Health != newPlayer.Health {
		delta.Health = &newPlayer.Health
	}
	if oldPlayer.MaxHealth != newPlayer.MaxHealth {
		delta.MaxHealth = &newPlayer.MaxHealth
	}

	// Compare leveling (changes occasionally/frequently)
	if oldPlayer.Level != newPlayer.Level {
		delta.Level = &newPlayer.Level
	}
	if oldPlayer.Experience != newPlayer.Experience {
		delta.Experience = &newPlayer.Experience
	}
	if oldPlayer.AvailableUpgrades != newPlayer.AvailableUpgrades {
		delta.AvailableUpgrades = &newPlayer.AvailableUpgrades
	}

	// Compare coins (changes with items/spending)
	if oldPlayer.Coins != newPlayer.Coins {
		delta.Coins = &newPlayer.Coins
	}

	// Compare upgrades (changes with stat upgrades)
	if !upgradesEqual(oldPlayer.Upgrades, newPlayer.Upgrades) {
		delta.Upgrades = &newPlayer.Upgrades
	}

	delta.ShipConfig = calculateShipConfigDeltas(&oldPlayer.ShipConfig, &newPlayer.ShipConfig)

	// Compare autofire (changes rarely)
	if oldPlayer.AutofireEnabled != newPlayer.AutofireEnabled {
		delta.AutofireEnabled = &newPlayer.AutofireEnabled
	}

	// Compare debug info (changes frequently for display)
	if !debugInfoEqual(oldPlayer.DebugInfo, newPlayer.DebugInfo) {
		delta.DebugInfo = &newPlayer.DebugInfo
	}

	return delta
}

// debugInfoEqual compares two DebugInfo structs
func debugInfoEqual(a, b DebugInfo) bool {
	return a.Health == b.Health &&
		a.RegenRate == b.RegenRate &&
		a.MoveSpeedModifier == b.MoveSpeedModifier &&
		a.TurnSpeedModifier == b.TurnSpeedModifier &&
		a.BodyDamage == b.BodyDamage &&
		a.FrontDPS == b.FrontDPS &&
		a.SideDPS == b.SideDPS &&
		a.RearDPS == b.RearDPS &&
		a.TopDPS == b.TopDPS &&
		a.TotalDPS == b.TotalDPS
}

// upgradesEqual compares two upgrade maps
func upgradesEqual(a, b map[UpgradeType]Upgrade) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, exists := b[key]
		if !exists {
			return false
		}
		if valA != valB {
			return false
		}
	}
	return true
}
