package game

import (
	"encoding/json"
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
		select {
		case <-ticker.C:
			w.update()
		}
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
	w.sendWelcomeMessage(client)

	// Send available upgrades
	sendAvailableUpgrades(client)

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

	// Handle respawning
	w.handleRespawns()

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

	// Periodic cleanup every 10 seconds (300 ticks at 30 TPS)
	if w.tickCounter%300 == 0 {
		w.performCleanup()
	}
}

// updatePlayer updates a single player's state with realistic ship physics
func (w *World) updatePlayer(player *Player, input *InputMsg) {
	// Handle respawn request if player is dead
	if player.State == StateDead && input.RequestRespawn {
		player.respawn()
		return
	}

	// Handle autofire toggle (works even in lobby)
	if input.ToggleAutofire {
		player.AutofireEnabled = !player.AutofireEnabled
		log.Printf("Player %d toggled autofire %s", player.ID, map[bool]string{true: "ON", false: "OFF"}[player.AutofireEnabled])
		input.ToggleAutofire = false // Clear input
	}

	// Handle stat upgrade purchases
	if input.StatUpgradeType != "" {
		statUpgradeType := UpgradeType(input.StatUpgradeType)
		if player.BuyUpgrade(statUpgradeType) {
			log.Printf("Player %d upgraded %s to level %d, coins remaining: %d",
				player.ID, statUpgradeType, player.Upgrades[statUpgradeType].Level, player.Coins)
		}
		input.StatUpgradeType = "" // Clear input
	}

	if player.State != StateAlive {
		return
	}

	// Handle thrust (W/S keys) - this affects speed, not direction
	var thrustForce float32 = 0
	if input.Up {
		thrustForce = ShipAcceleration
	}
	if input.Down {
		thrustForce = -ShipAcceleration * 0.5 // Reverse is weaker
	}

	// Apply thrust in the direction the ship is facing
	if thrustForce != 0 {
		thrustX := float32(math.Cos(float64(player.Angle))) * thrustForce
		thrustY := float32(math.Sin(float64(player.Angle))) * thrustForce
		player.VelX += thrustX
		player.VelY += thrustY
	}

	// Calculate max speed with move speed upgrade and hull strength reduction
	maxSpeed := (BaseShipMaxSpeed * player.Modifiers.MoveSpeedMultiplier)
	if !player.IsBot {
		//fmt.Println("Player", player.ID, "max speed:", maxSpeed)
	}
	speed := min(float32(math.Sqrt(float64(player.VelX*player.VelX+player.VelY*player.VelY))), maxSpeed)

	// Scale turn speed based on current speed and ship length
	// Example: turn faster at low speed, slower at high speed
	// Longer ships turn slower (more realistic naval physics)
	turnFactor := speed / BaseShipMaxSpeed

	// Calculate length factor - longer ships turn slower
	// Base length for comparison (1 cannon = standard ship)
	baseShipLength := float32(PlayerSize * 1.2)                   // 1 cannon ship has no length multiplier
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
	newSpeed := float32(math.Sqrt(float64(player.VelX*player.VelX + player.VelY*player.VelY)))
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

	// Handle ship upgrades - use new modular system
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
	if input.UpgradeScatter {
		player.ShipConfig.SideUpgrade = NewScatterSideCannons(player.ShipConfig.SideUpgrade.Count + 1)
		player.ShipConfig.CalculateShipDimensions()
		player.ShipConfig.UpdateUpgradePositions()
	}
	if input.DowngradeScatter {
		player.ShipConfig.SideUpgrade = NewScatterSideCannons(player.ShipConfig.SideUpgrade.Count - 1)
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
			sendAvailableUpgrades(client)
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
					sendAvailableUpgrades(client)
				}
			}
		}

		// Clear upgrade input to prevent multiple upgrades per frame
		input.SelectUpgrade = ""
		input.UpgradeChoice = ""
	}

	// Handle health regeneration from auto repairs upgrade
	// Regenerate health based on time elapsed
	elapsedSeconds := float32(now.Sub(player.LastRegenTime).Seconds())
	if elapsedSeconds >= 0.2 {
		healthToRegen := int(elapsedSeconds * player.Modifiers.HealthRegenPerSec)
		if healthToRegen > 0 && player.Health < player.MaxHealth {
			player.Health += healthToRegen
			if player.Health > player.MaxHealth {
				player.Health = player.MaxHealth
			}
			player.LastRegenTime = now
		}

	}

	// Keep player within world boundaries
	w.keepPlayerInBounds(player)
}

// performCleanup removes old entities and prevents memory leaks
func (w *World) performCleanup() {
	now := time.Now()

	// Clean up old bullets (in case some weren't removed properly)
	oldBullets := 0
	for id, bullet := range w.bullets {
		if now.Sub(bullet.CreatedAt).Seconds() > BulletLifetime+5 { // 5 second grace period
			delete(w.bullets, id)
			oldBullets++
		}
	}

	// Limit total bullets to prevent memory issues
	if len(w.bullets) > 1000 {
		// Remove oldest bullets
		type bulletAge struct {
			id  uint32
			age time.Duration
		}
		bulletAges := make([]bulletAge, 0, len(w.bullets))
		for id, bullet := range w.bullets {
			bulletAges = append(bulletAges, bulletAge{id, now.Sub(bullet.CreatedAt)})
		}

		// Sort by age (oldest first)
		for i := 0; i < len(bulletAges)-1; i++ {
			for j := i + 1; j < len(bulletAges); j++ {
				if bulletAges[i].age < bulletAges[j].age {
					bulletAges[i], bulletAges[j] = bulletAges[j], bulletAges[i]
				}
			}
		}

		// Remove oldest bullets to get under limit
		toRemove := len(w.bullets) - 800 // Keep 800, remove excess
		for i := 0; i < toRemove && i < len(bulletAges); i++ {
			delete(w.bullets, bulletAges[i].id)
		}
		log.Printf("Cleaned up %d excess bullets", toRemove)
	}

	// Limit total items to prevent server overload
	if len(w.items) > 500 {
		// Remove oldest items
		itemsToRemove := len(w.items) - 400 // Keep 400, remove excess
		count := 0
		for id := range w.items {
			if count >= itemsToRemove {
				break
			}
			delete(w.items, id)
			count++
		}
		log.Printf("Cleaned up %d excess items", itemsToRemove)
	}

	if oldBullets > 0 {
		log.Printf("Cleanup: removed %d old bullets, %d total bullets, %d total items",
			oldBullets, len(w.bullets), len(w.items))
	}
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

// handleRespawns checks for dead players that need to respawn
func (w *World) handleRespawns() {
	now := time.Now()
	for _, player := range w.players {
		if player.State == StateDead && now.After(player.RespawnTime) {
			if player.IsBot {
				if bot, exists := w.bots[player.ID]; exists {
					w.respawnBot(bot, now)
				}
				continue
			}

			// For human players, don't auto-respawn - wait for their respawn request
			// The respawn is handled in processInput when RequestRespawn is true
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

// broadcastSnapshot sends the current game state to all clients (optimized)
func (w *World) broadcastSnapshot() {
	// Limit data to reduce bandwidth
	maxItems := MaxItems * 2
	maxBullets := 300

	snapshot := Snapshot{
		Type:    MsgTypeSnapshot,
		Players: make([]Player, 0, len(w.players)),
		Items:   make([]GameItem, 0, min(len(w.items), maxItems)),
		Bullets: make([]Bullet, 0, min(len(w.bullets), maxBullets)),
		Time:    time.Now().UnixMilli(),
	}

	// Add all players to snapshot
	for _, player := range w.players {
		snapshot.Players = append(snapshot.Players, *player)
	}

	// Add limited items to snapshot (prioritize closer items for performance)
	itemCount := 0
	for _, item := range w.items {
		if itemCount >= maxItems {
			break
		}
		snapshot.Items = append(snapshot.Items, *item)
		itemCount++
	}

	// Add limited bullets to snapshot
	bulletCount := 0
	for _, bullet := range w.bullets {
		if bulletCount >= maxBullets {
			break
		}
		snapshot.Bullets = append(snapshot.Bullets, *bullet)
		bulletCount++
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Error marshaling snapshot: %v", err)
		return
	}

	// Send to all clients concurrently (non-blocking)
	for _, client := range w.clients {
		go func(c *Client) {
			select {
			case c.Send <- data:
			case <-time.After(10 * time.Millisecond):
				// Skip slow clients to prevent blocking
			}
		}(client)
	}
}

// sendWelcomeMessage sends a welcome message to a specific client with their player ID
func (w *World) sendWelcomeMessage(client *Client) {
	welcomeMsg := WelcomeMsg{
		Type:     MsgTypeWelcome,
		PlayerId: client.ID,
	}

	data, err := json.Marshal(welcomeMsg)
	if err != nil {
		log.Printf("Error marshaling welcome message: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		// Channel full, skip
		log.Printf("Could not send welcome message to client %d", client.ID)
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
	player.X = float32(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldWidth-player.ShipConfig.Size/2), float64(player.X))))
	player.Y = float32(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldHeight-player.ShipConfig.Size/2), float64(player.Y))))
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

		// Remove bullets that are out of bounds
		if bullet.X < -100 || bullet.X > WorldWidth+100 || bullet.Y < -100 || bullet.Y > WorldHeight+100 {
			bulletsToDelete = append(bulletsToDelete, id)
			continue
		}

		// Check collision with players (only if bullet is in world bounds)
		bulletHit := false
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
					damage = BulletDamage // Fallback to default for legacy bullets
				}
				w.mechanics.ApplyDamage(player, damage, attacker, KillCauseBullet, now)

				// Mark bullet for deletion
				bulletsToDelete = append(bulletsToDelete, id)
				bulletHit = true

				break // Bullet hit something, stop checking other players
			}
		}

		if bulletHit {
			break // Move to next bullet
		}
	}

	// Delete bullets in batch (avoid map modification during iteration)
	for _, bulletID := range bulletsToDelete {
		delete(w.bullets, bulletID)
	}
}

// checkBulletPlayerCollision checks if a bullet collides with a player using rectangular bounding boxes
func (w *World) checkBulletPlayerCollision(bullet *Bullet, player *Player) bool {
	// Get player's bounding box using the mechanics instance
	playerBbox := w.mechanics.GetShipBoundingBox(player)

	// Create bullet bounding box (treat bullet as a small rectangle)
	bulletHalfSize := bullet.Size / 2
	bulletBbox := BoundingBox{
		MinX: bullet.X - bulletHalfSize,
		MinY: bullet.Y - bulletHalfSize,
		MaxX: bullet.X + bulletHalfSize,
		MaxY: bullet.Y + bulletHalfSize,
	}

	// Check if bounding boxes overlap
	return bulletBbox.MinX < playerBbox.MaxX && bulletBbox.MaxX > playerBbox.MinX &&
		bulletBbox.MinY < playerBbox.MaxY && bulletBbox.MaxY > playerBbox.MinY
}

// checkPlayerItemCollision checks if a player collides with an item using rectangular bounding boxes
func (w *World) checkPlayerItemCollision(player *Player, item *GameItem) bool {
	// Get player's bounding box using the mechanics instance
	playerBbox := w.mechanics.GetShipBoundingBox(player)

	// Create item bounding box (treat item as a small rectangle)
	itemHalfSize := float32(ItemPickupSize) / 2
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
