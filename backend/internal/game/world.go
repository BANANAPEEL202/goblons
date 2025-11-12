package game

import (
	"log"
	"math"
	"time"
)

// NewWorld creates a new game world
func NewWorld() *World {
	world := &World{
		clients:      make(map[uint32]*Client),
		players:      make(map[uint32]*Player),
		bots:         make(map[uint32]*Bot),
		items:        make(map[uint32]*GameItem),
		bullets:      make(map[uint32]*Bullet),
		nextPlayerID: 1,
		itemID:       1,
		bulletID:     1,
		running:      false,
	}
	world.mechanics = NewGameMechanics(world)
	return world
}

// Start begins the game loop
func (w *World) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	// Spawn persistent bots before the game loop begins
	w.spawnInitialBots()

	// Spawn initial items
	go w.spawnItems()

	// Main game loop
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()

	log.Println("Game world started")
	for w.running {
		<-ticker.C
		w.update()
	}

}

// Stop stops the game world
func (w *World) Stop() {
	w.mu.Lock()
	w.running = false
	w.mu.Unlock()
}

// AddClient adds a new client to the world with connection limits
func (w *World) AddClient(client *Client) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check player limit for performance
	if len(w.clients) >= MaxPlayers {
		log.Printf("Server full: rejecting new player (limit: %d)", MaxPlayers)
		return false
	}

	client.ID = w.nextPlayerID
	client.Player.ID = w.nextPlayerID
	w.nextPlayerID++

	w.clients[client.ID] = client
	w.players[client.ID] = client.Player

	// Keep player in dead state until they press "Set Sail"
	client.Player.State = StateDead

	// Initialize ship dimensions and weapon positions (but don't spawn yet)
	client.Player.updateShipGeometry()

	// Send welcome message to the new client with their player ID
	client.sendWelcomeMessage()

	// Send available upgrades
	client.sendAvailableUpgrades()

	log.Printf("Player %d (%s) joined the lobby (%d/%d players)", client.ID, client.Player.Name, len(w.clients), MaxPlayers)
	return true
}

// RemoveClient removes a client from the world
func (w *World) RemoveClient(clientID uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if client, exists := w.clients[clientID]; exists {
		log.Printf("Player %d (%s) left the game", clientID, client.Player.Name)
		close(client.Send)
		delete(w.clients, clientID)
		delete(w.players, clientID)
	}
}

// GetClient returns a client by ID
func (w *World) GetClient(id uint32) (*Client, bool) {
	client, exists := w.clients[id]
	return client, exists
}

// update runs one game tick
func (w *World) update() {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Update all players
	for _, player := range w.players {
		if player.IsBot {
			continue
		}
		if client, exists := w.clients[player.ID]; exists {
			w.updatePlayer(player, &client.Input)
		}
	}

	// Update bot-controlled ships using AI inputs
	w.updateBots()

	// Update bullets
	w.updateBullets()

	// Check collisions
	w.checkCollisions()

	// Handle player vs player collisions
	w.mechanics.HandlePlayerCollisions()

	// Send snapshot to all clients (only every other tick for performance)
	w.tickCounter++
	if w.tickCounter%1 == 0 {
		w.broadcastSnapshot()
	}
}

// processPlayerActions handles event-based actions with deduplication and cooldowns
func (w *World) processPlayerActions(player *Player, input *InputMsg) {
	now := time.Now()

	// Define cooldowns for each action type
	actionCooldowns := map[string]time.Duration{
		"statUpgrade":    100 * time.Millisecond,
		"toggleAutofire": 400 * time.Millisecond,
	}

	for _, action := range input.Actions {
		// Skip if this action was already processed (deduplication)
		if action.Sequence <= player.LastProcessedAction {
			log.Printf("Player %d skipping already processed action seq %d (last: %d)",
				player.ID, action.Sequence, player.LastProcessedAction)
			// Update last processed to prevent reprocessing this sequence
			player.LastProcessedAction = action.Sequence
			continue
		}

		// Check cooldown for this action type
		if lastTime, exists := player.ActionCooldowns[action.Type]; exists {
			cooldown := actionCooldowns[action.Type]
			elapsed := now.Sub(lastTime)
			if elapsed < cooldown {
				log.Printf("Player %d action %s on cooldown (elapsed: %dms, need: %dms), skipping seq %d",
					player.ID, action.Type, elapsed.Milliseconds(), cooldown.Milliseconds(), action.Sequence)
				// Still update last processed to avoid reprocessing
				player.LastProcessedAction = action.Sequence
				continue
			}
		}

		// Process the action
		handled := false
		switch action.Type {
		case "statUpgrade":
			statUpgradeType := UpgradeType(action.Data)
			if player.BuyUpgrade(statUpgradeType) {
				log.Printf("Player %d upgraded %s to level %d, coins remaining: %d (seq: %d)",
					player.ID, statUpgradeType, player.Upgrades[statUpgradeType].Level, player.Coins, action.Sequence)
				handled = true
			} else {
				log.Printf("Player %d failed to upgrade %s (seq: %d)", player.ID, statUpgradeType, action.Sequence)
			}

		case "toggleAutofire":
			player.AutofireEnabled = !player.AutofireEnabled
			log.Printf("Player %d toggled autofire %s (seq: %d)", player.ID,
				map[bool]string{true: "ON", false: "OFF"}[player.AutofireEnabled], action.Sequence)
			handled = true
		}

		// Always update last processed sequence to avoid reprocessing
		player.LastProcessedAction = action.Sequence

		// Update cooldown only if action was successfully handled
		if handled {
			player.ActionCooldowns[action.Type] = now
		}
	}
}

// updatePlayer updates a single player's state with realistic ship physics
func (w *World) updatePlayer(player *Player, input *InputMsg) {
	// Handle respawn request if player is dead
	if player.State == StateDead && input.RequestRespawn {
		player.respawn()
		return
	}

	// Process new action-based inputs
	w.processPlayerActions(player, input)

	// Handle legacy inputs for backward compatibility
	if input.ToggleAutofire {
		player.AutofireEnabled = !player.AutofireEnabled
		log.Printf("Player %d toggled autofire %s", player.ID, map[bool]string{true: "ON", false: "OFF"}[player.AutofireEnabled])
		input.ToggleAutofire = false
	}

	if input.StatUpgradeType != "" {
		statUpgradeType := UpgradeType(input.StatUpgradeType)
		if player.BuyUpgrade(statUpgradeType) {
			log.Printf("Player %d upgraded %s to level %d, coins remaining: %d",
				player.ID, statUpgradeType, player.Upgrades[statUpgradeType].Level, player.Coins)
		}
		input.StatUpgradeType = ""
	}

	if player.State != StateAlive {
		return
	}

	// Calculate max speed with move speed upgrade and hull strength reduction
	maxSpeed := (BaseShipMaxSpeed * player.Modifiers.MoveSpeedMultiplier)
	if input.Up {
		player.VelX = float64(math.Cos(float64(player.Angle))) * maxSpeed
		player.VelY = float64(math.Sin(float64(player.Angle))) * maxSpeed
	}
	speed := min(float64(math.Sqrt(float64(player.VelX*player.VelX+player.VelY*player.VelY))), maxSpeed)

	// Scale turn speed based on current speed and ship length
	// Example: turn faster at low speed, slower at high speed
	// Longer ships turn slower (more realistic naval physics)
	turnFactor := speed / BaseShipMaxSpeed

	// Calculate length factor - longer ships turn slower
	// Base length for comparison (1 cannon = standard ship)
	baseShipLength := float64(PlayerSize * 1.2)                   // 1 cannon ship has no length multiplier
	lengthFactor := baseShipLength / player.ShipConfig.ShipLength // Longer ships get smaller factor

	// Apply turn speed upgrade
	baseTurnSpeed := BaseShipTurnSpeed * player.Modifiers.TurnSpeedMultiplier
	scaledTurnSpeed := baseTurnSpeed * turnFactor * lengthFactor

	// Handle turning (A/D keys) and track angular velocity
	if input.Left {
		player.Angle -= scaledTurnSpeed
	}
	if input.Right {
		player.Angle += scaledTurnSpeed
	}

	// Apply drag/deceleration
	player.VelX *= ShipDeceleration
	player.VelY *= ShipDeceleration

	// Limit maximum speed
	newSpeed := float64(math.Sqrt(float64(player.VelX*player.VelX + player.VelY*player.VelY)))
	if newSpeed > maxSpeed {
		speedRatio := maxSpeed / newSpeed
		player.VelX *= speedRatio
		player.VelY *= speedRatio
	}

	// Update position
	player.X += player.VelX
	player.Y += player.VelY

	// Update turret aiming and firing using modular system
	now := time.Now()
	w.updateModularTurretAiming(player, input)
	w.fireModularUpgrades(player, input, now)

	for player.Experience >= player.GetExperienceRequiredForNextLevel() {
		player.Level++
		player.AvailableUpgrades++
	}

	if DEV {
		if input.UpgradeCannons {
			player.ShipConfig.SideUpgrade = NewBasicSideCannons(player.ShipConfig.SideUpgrade.Count + 1)
			player.ShipConfig.CalculateShipDimensions()
			player.ShipConfig.UpdateUpgradePositions()
		}
		if input.DowngradeCannons {
			player.ShipConfig.SideUpgrade = NewBasicSideCannons(player.ShipConfig.SideUpgrade.Count - 1)
			player.ShipConfig.CalculateShipDimensions()
			player.ShipConfig.UpdateUpgradePositions()
		}
		if input.UpgradeTurrets {
			player.ShipConfig.TopUpgrade = NewBasicTurrets(player.ShipConfig.TopUpgrade.Count + 1)
			player.ShipConfig.CalculateShipDimensions()
			player.ShipConfig.UpdateUpgradePositions()
		}
		if input.DowngradeTurrets {
			player.ShipConfig.TopUpgrade = NewBasicTurrets(player.ShipConfig.TopUpgrade.Count - 1)
			player.ShipConfig.CalculateShipDimensions()
			player.ShipConfig.UpdateUpgradePositions()
		}

		// Handle leveling system
		if input.DebugLevelUp {
			player.DebugLevelUp()
			// Send updated available upgrades to client
			if client, exists := w.GetClient(player.ID); exists {
				client.sendAvailableUpgrades()
			}
		}
	}

	// Handle upgrade selection (only one upgrade per level with cooldown protection)
	if input.SelectUpgrade != "" && input.UpgradeChoice != "" && player.AvailableUpgrades > 0 {
		// Get client for cooldown check
		if client, exists := w.GetClient(player.ID); exists {
			now := time.Now()

			// Enforce upgrade cooldown (500ms between upgrades)
			if now.Sub(client.LastUpgrade) < 500*time.Millisecond {
				// Clear input and skip processing
				input.SelectUpgrade = ""
				input.UpgradeChoice = ""
				return
			}

			var upgradeType moduleType
			switch input.SelectUpgrade {
			case "side":
				upgradeType = UpgradeTypeSide
			case "top":
				upgradeType = UpgradeTypeTop
			case "front":
				upgradeType = UpgradeTypeFront
			case "rear":
				upgradeType = UpgradeTypeRear
			default:
				upgradeType = ""
			}

			if upgradeType != "" {
				if player.ShipConfig.ApplyModule(upgradeType, input.UpgradeChoice) {
					player.updateModifiers()
					player.AvailableUpgrades--
					client.LastUpgrade = now // Update last upgrade time
					log.Printf("Player %d applied upgrade %s:%s, remaining upgrades: %d",
						player.ID, upgradeType, input.UpgradeChoice, player.AvailableUpgrades)
					// Send updated available upgrades to client
					client.sendAvailableUpgrades()
				}
			}
		}

		// Clear upgrade input to prevent multiple upgrades per frame
		input.SelectUpgrade = ""
		input.UpgradeChoice = ""
	}

	// Handle health regeneration from auto repairs upgrade
	// Regenerate health based on time elapsed
	elapsedSeconds := float64(1 / TickRate)
	healthToRegen := int(elapsedSeconds * player.Modifiers.HealthRegenPerSec)
	if healthToRegen > 0 && player.Health < player.MaxHealth {
		player.Health += healthToRegen
		if player.Health > player.MaxHealth {
			player.Health = player.MaxHealth
		}
	}

	// Keep player within world boundaries
	w.keepPlayerInBounds(player)
}

// checkCollisions handles player-item collisions (optimized)
func (w *World) checkCollisions() {
	// Early exit if no items or players
	if len(w.items) == 0 || len(w.players) == 0 {
		return
	}

	// Pre-allocate slice for items to collect (avoid map iteration during deletion)
	itemsToCollect := make([]struct{ playerID, itemID uint32 }, 0, 16)

	for playerID, player := range w.players {
		if player.State != StateAlive {
			continue
		}

		// Simple distance check first (cheaper than full bounding box)
		for itemID, item := range w.items {
			// Quick distance check (using squares to avoid sqrt)
			dx := player.X - item.X
			dy := player.Y - item.Y
			distSq := dx*dx + dy*dy

			// Only do expensive collision check if close enough
			if distSq < 2500 && w.checkPlayerItemCollision(player, item) { // 50^2 = 2500
				itemsToCollect = append(itemsToCollect, struct{ playerID, itemID uint32 }{playerID, itemID})
			}
		}
	}

	// Process collections after iteration to avoid map modification during iteration
	for _, collision := range itemsToCollect {
		if _, exists := w.players[collision.playerID]; exists {
			if _, exists := w.items[collision.itemID]; exists {
				w.collectItem(collision.playerID, collision.itemID)
			}
		}
	}
}

// collectItem handles when a player collects an item
func (w *World) collectItem(playerID, itemID uint32) {
	player, playerExists := w.players[playerID]
	item, itemExists := w.items[itemID]
	if !playerExists || !itemExists {
		return
	}

	// Use the mechanics system to apply item effects
	w.mechanics.ApplyItemEffect(player, item)

	delete(w.items, itemID)
}

// handleBotRespawns checks for dead players that need to respawn
func (w *World) handleBotRespawns() {
	now := time.Now()
	for _, player := range w.players {
		if player.IsBot {
			if player.State == StateDead && now.After(player.RespawnTime) {
				if bot, exists := w.bots[player.ID]; exists {
					w.respawnBot(bot, now)
				}
				continue
			}
		}
	}

}

// spawnItems continuously spawns items in the world (with limits)
func (w *World) spawnItems() {
	foodTicker := time.NewTicker(time.Second * 2)     // Spawn food every 2 seconds (reduced frequency)
	specialTicker := time.NewTicker(time.Second * 10) // Spawn special items every 10 seconds (reduced frequency)
	defer foodTicker.Stop()
	defer specialTicker.Stop()

	for w.running {
		select {
		case <-foodTicker.C:
			w.mu.Lock()
			// Reduced item limit and spawn rate to prevent accumulation
			if len(w.items) < MaxItems && len(w.players) > 0 { // Only spawn if players present
				w.mechanics.SpawnFoodItems()
			}
			w.mu.Unlock()
		case <-specialTicker.C:
			w.mu.Lock()
			// Only spawn special items occasionally
			if len(w.items) < 75 && len(w.players) > 2 { // Only if multiple players
				w.mechanics.SpawnFoodItems() // Reuse food spawning for now
			}
			w.mu.Unlock()
		}
	}
}

// HandleInput processes input from a client
func (w *World) HandleInput(clientID uint32, input InputMsg) {
	client, exists := w.GetClient(clientID)
	if !exists {
		return
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	switch input.Type {
	case "profile":
		if sanitizedName := SanitizePlayerName(input.PlayerName); sanitizedName != "" {
			client.Player.Name = sanitizedName
		}
		if sanitizedColor := SanitizePlayerColor(input.PlayerColor); sanitizedColor != "" {
			client.Player.Color = sanitizedColor
		}
	case "startGame":
		// When player presses "Set Sail", spawn them into the game
		if client.Player.State == StateDead && input.StartGame {
			client.Player.spawn()
			log.Printf("Player %d (%s) set sail and entered the game", client.ID, client.Player.Name)
		}
	default:
		client.Input = input
	}

	client.LastSeen = time.Now()
}

// keepPlayerInBounds ensures a player stays within the world boundaries
func (w *World) keepPlayerInBounds(player *Player) {
	player.X = float64(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldWidth-player.ShipConfig.Size/2), float64(player.X))))
	player.Y = float64(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldHeight-player.ShipConfig.Size/2), float64(player.Y))))
}

// updateBullets handles bullet movement and cleanup (optimized)
func (w *World) updateBullets() {
	if len(w.bullets) == 0 {
		return
	}

	now := time.Now()
	bulletsToDelete := make([]uint32, 0, 32) // Pre-allocate for common case

	for id, bullet := range w.bullets {
		// Check if bullet has expired
		if now.Sub(bullet.CreatedAt).Seconds() >= BulletLifetime {
			bulletsToDelete = append(bulletsToDelete, id)
			continue
		}

		// Update bullet position
		bullet.X += bullet.VelX
		bullet.Y += bullet.VelY

		// skip out of bounds bullets
		if bullet.X < -100 || bullet.X > WorldWidth+100 || bullet.Y < -100 || bullet.Y > WorldHeight+100 {
			continue
		}

		// Check collision with players (only if bullet is in world bounds)
		var attacker *Player
		if shooter, exists := w.players[bullet.OwnerID]; exists {
			attacker = shooter
		}
		for playerID, player := range w.players {
			// Skip if bullet owner or player is dead
			if bullet.OwnerID == playerID || player.State != StateAlive {
				continue
			}

			// Quick distance check before expensive bounding box collision
			dx := bullet.X - player.X
			dy := bullet.Y - player.Y
			distSq := dx*dx + dy*dy

			// Only do expensive collision check if close enough (player size + some margin)
			if distSq < 10000 && w.checkBulletPlayerCollision(bullet, player) { // 100^2 = 10000
				// Apply damage through mechanics system (handles death + rewards)
				damage := bullet.Damage * int(attacker.Modifiers.BulletDamageMultiplier)
				if damage == 0 {
					damage = BulletDamage
					log.Printf("Bullet damage calculated as 0 for player %d, defaulting to %d", attacker.ID, BulletDamage)
				}
				w.mechanics.ApplyDamage(player, damage, attacker, KillCauseBullet, now)

				// Mark bullet for deletion
				bulletsToDelete = append(bulletsToDelete, id)

				break // Bullet hit something, stop checking other players
			}
		}
	}

	// Delete bullets in batch (avoid map modification during iteration)
	for _, bulletID := range bulletsToDelete {
		delete(w.bullets, bulletID)
	}
}

// checkBulletPlayerCollision checks if a bullet collides with a player using rectangular bounding boxes
func (w *World) checkBulletPlayerCollision(bullet *Bullet, player *Player) bool {
	playerBbox := player.GetShipBoundingBox()

	// Bullet treated as a circle
	cx, cy := bullet.X, bullet.Y

	// Find the closest point on the rectangle to the bullet center
	closestX := math.Max(playerBbox.MinX, math.Min(cx, playerBbox.MaxX))
	closestY := math.Max(playerBbox.MinY, math.Min(cy, playerBbox.MaxY))

	// Compute distance from bullet center to that closest point
	dx := cx - closestX
	dy := cy - closestY
	distSq := dx*dx + dy*dy

	return distSq <= bullet.Radius*bullet.Radius
}

// checkPlayerItemCollision checks if a player collides with an item using rectangular bounding boxes
func (w *World) checkPlayerItemCollision(player *Player, item *GameItem) bool {
	// Get player's bounding box using the mechanics instance
	playerBbox := player.GetShipBoundingBox()

	// Create item bounding box (treat item as a small rectangle)
	itemHalfSize := float64(ItemPickupSize) / 2
	itemBbox := BoundingBox{
		MinX: item.X - itemHalfSize,
		MinY: item.Y - itemHalfSize,
		MaxX: item.X + itemHalfSize,
		MaxY: item.Y + itemHalfSize,
	}

	// Check if bounding boxes overlap
	return itemBbox.MinX < playerBbox.MaxX && itemBbox.MaxX > playerBbox.MinX &&
		itemBbox.MinY < playerBbox.MaxY && itemBbox.MaxY > playerBbox.MinY
}

// fireModularUpgrades fires weapons based on upgrade categories with per-category cooldowns
func (w *World) fireModularUpgrades(player *Player, input *InputMsg, now time.Time) {
	// Fire if autofire is enabled OR if manual fire is triggered
	if !player.AutofireEnabled && !input.ManualFire {
		return
	}

	// Clear manual fire flag after processing
	if input.ManualFire {
		input.ManualFire = false
	}

	w.fireSideUpgrade(player, now)
	w.fireTopUpgrade(player, now)
	w.fireFrontUpgrade(player, now)
	w.fireRearUpgrade(player, now)
}

// registerBullets adds the emitted bullets to the world map in one place.
func (w *World) registerBullets(bullets []*Bullet) {
	for _, bullet := range bullets {
		w.bullets[bullet.ID] = bullet
	}
}

// fireCannons iterates a list of cannons and fires them using their configured angles.
func (w *World) fireCannons(player *Player, cannons []*Cannon, now time.Time) bool {
	fired := false
	for _, cannon := range cannons {
		// Skip non-firing equipment such as oars
		if cannon.Type == WeaponTypeRow {
			continue
		}

		angle := player.Angle + cannon.Angle
		bullets := cannon.Fire(w, player, angle, now)
		if len(bullets) == 0 {
			continue
		}

		w.registerBullets(bullets)
		fired = true
	}

	return fired
}

// fireTurrets iterates a list of turrets and registers emitted bullets.
func (w *World) fireTurrets(player *Player, turrets []*Turret, now time.Time) bool {
	fired := false
	for i := range turrets {
		bullets := turrets[i].Fire(w, player, now)
		if len(bullets) == 0 {
			continue
		}

		w.registerBullets(bullets)
		fired = true
	}

	return fired
}

// fireSideUpgrade fires side-mounted cannons from the single side upgrade
func (w *World) fireSideUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.SideUpgrade == nil {
		return false
	}

	upgrade := player.ShipConfig.SideUpgrade
	if upgrade.Type != UpgradeTypeSide {
		return false
	}

	cannonCount := len(upgrade.Cannons) / 2
	if cannonCount == 0 {
		return false
	}

	return w.fireCannons(player, upgrade.Cannons, now)
}

// fireTopUpgrade fires top-mounted turrets from the single top upgrade
func (w *World) fireTopUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.TopUpgrade == nil || player.ShipConfig.TopUpgrade.Type != UpgradeTypeTop {
		return false
	}

	upgrade := player.ShipConfig.TopUpgrade
	return w.fireTurrets(player, upgrade.Turrets, now)
}

// fireFrontUpgrade fires front-mounted weapons from the single front upgrade
func (w *World) fireFrontUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.FrontUpgrade == nil || player.ShipConfig.FrontUpgrade.Type != UpgradeTypeFront {
		return false
	}

	upgrade := player.ShipConfig.FrontUpgrade
	firedCannons := w.fireCannons(player, upgrade.Cannons, now)
	firedTurrets := w.fireTurrets(player, upgrade.Turrets, now)

	return firedCannons || firedTurrets
}

// fireRearUpgrade fires rear-mounted weapons from the single rear upgrade
func (w *World) fireRearUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.RearUpgrade == nil || player.ShipConfig.RearUpgrade.Type != UpgradeTypeRear {
		return false
	}

	upgrade := player.ShipConfig.RearUpgrade
	firedCannons := w.fireCannons(player, upgrade.Cannons, now)
	firedTurrets := w.fireTurrets(player, upgrade.Turrets, now)

	return firedCannons || firedTurrets
}

// updateModularTurretAiming updates turret aiming using the new modular system
func (w *World) updateModularTurretAiming(player *Player, input *InputMsg) {
	mouseWorldX := input.Mouse.X
	mouseWorldY := input.Mouse.Y

	// Update turrets in all upgrade categories
	upgrades := []*ShipModule{player.ShipConfig.TopUpgrade, player.ShipConfig.FrontUpgrade, player.ShipConfig.RearUpgrade}

	for _, upgrade := range upgrades {
		if upgrade != nil {
			for i := range upgrade.Turrets {
				turret := upgrade.Turrets[i]
				turret.UpdateAiming(player, mouseWorldX, mouseWorldY)
			}
		}
	}
}

// calculateDebugInfo computes debug values for client display
func (w *World) calculateDebugInfo(player *Player) DebugInfo {
	baseShipLength := float64(PlayerSize * 1.2)                   // 1 cannon ship has no length multiplier
	lengthFactor := baseShipLength / player.ShipConfig.ShipLength // Longer ships get smaller factor
	debugInfo := DebugInfo{
		Health:            player.MaxHealth,
		RegenRate:         player.Modifiers.HealthRegenPerSec,
		MoveSpeedModifier: player.Modifiers.MoveSpeedMultiplier,
		TurnSpeedModifier: player.Modifiers.TurnSpeedMultiplier * lengthFactor,
		BodyDamage:        player.Modifiers.BodyDamageBonus,
		FrontDPS:          0,
		SideDPS:           0,
		RearDPS:           0,
		TopDPS:            0,
		TotalDPS:          0,
	}

	// Calculate DPS from all cannons
	cannonDamageMod := player.Modifiers.BulletDamageMultiplier
	reloadSpeedMod := player.Modifiers.ReloadSpeedMultiplier

	// Calculate DPS for each upgrade type
	if player.ShipConfig.FrontUpgrade != nil {
		for _, cannon := range player.ShipConfig.FrontUpgrade.Cannons {
			damage := float64(cannon.Stats.BulletDamageMod * BulletDamage)
			reloadRate := cannon.Stats.ReloadTime
			effectiveDamage := damage * (cannonDamageMod)
			effectiveReloadRate := reloadRate * (reloadSpeedMod)
			if effectiveReloadRate > 0 {
				debugInfo.FrontDPS += effectiveDamage * 1 / effectiveReloadRate
			}
		}
	}

	if player.ShipConfig.SideUpgrade != nil {
		for _, cannon := range player.ShipConfig.SideUpgrade.Cannons {
			damage := float64(cannon.Stats.BulletDamageMod * BulletDamage)
			reloadRate := cannon.Stats.ReloadTime
			effectiveDamage := damage * (cannonDamageMod)
			effectiveReloadRate := reloadRate * (reloadSpeedMod)
			if effectiveReloadRate > 0 {
				debugInfo.SideDPS += effectiveDamage * 1 / effectiveReloadRate
			}
		}
	}

	if player.ShipConfig.RearUpgrade != nil {
		for _, cannon := range player.ShipConfig.RearUpgrade.Cannons {
			damage := float64(cannon.Stats.BulletDamageMod * BulletDamage)
			reloadRate := cannon.Stats.ReloadTime
			effectiveDamage := damage * (cannonDamageMod)
			effectiveReloadRate := reloadRate * (reloadSpeedMod)
			if effectiveReloadRate > 0 {
				debugInfo.RearDPS += effectiveDamage * 1 / effectiveReloadRate
			}
		}
	}

	if player.ShipConfig.TopUpgrade != nil {
		for _, turret := range player.ShipConfig.TopUpgrade.Turrets {
			// only calculated based on first cannon
			// machine gun dual cannon shares reload
			turretCannon := turret.Cannons[0]

			damage := float64(turretCannon.Stats.BulletDamageMod * BulletDamage)
			reloadRate := turretCannon.Stats.ReloadTime
			effectiveDamage := damage * (cannonDamageMod)
			effectiveReloadRate := reloadRate * (reloadSpeedMod)
			if effectiveReloadRate > 0 {
				debugInfo.TopDPS += effectiveDamage * 1 / effectiveReloadRate
			}
		}
	}

	debugInfo.TotalDPS = debugInfo.FrontDPS + debugInfo.SideDPS + debugInfo.RearDPS + debugInfo.TopDPS

	return debugInfo
}
