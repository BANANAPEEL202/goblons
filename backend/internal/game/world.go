package game

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
)

// NewWorld creates a new game world
func NewWorld() *World {
	world := &World{
		clients:  make(map[uint32]*Client),
		players:  make(map[uint32]*Player),
		items:    make(map[uint32]*GameItem),
		bullets:  make(map[uint32]*Bullet),
		nextID:   1,
		itemID:   1,
		bulletID: 1,
		running:  false,
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

// AddClient adds a new client to the world
func (w *World) AddClient(client *Client) {
	w.mu.Lock()
	defer w.mu.Unlock()

	client.ID = w.nextID
	client.Player.ID = w.nextID
	w.nextID++

	w.clients[client.ID] = client
	w.players[client.ID] = client.Player

	// Spawn player at random safe location
	w.spawnPlayer(client.Player)

	// Initialize ship dimensions and weapon positions
	w.updateShipDimensions(client.Player)

	// Send welcome message to the new client with their player ID
	w.sendWelcomeMessage(client)

	// Send available upgrades
	w.sendAvailableUpgrades(client)

	log.Printf("Player %d (%s) joined the game", client.ID, client.Player.Name)
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
		if client, exists := w.clients[player.ID]; exists {
			w.updatePlayer(player, &client.Input)
		}
	}

	// Handle respawning
	w.handleRespawns()

	// Update bullets
	w.updateBullets()

	// Check collisions
	w.checkCollisions()

	// Handle player vs player collisions
	w.mechanics.HandlePlayerCollisions()

	// Send snapshot to all clients
	w.broadcastSnapshot()
}

// updatePlayer updates a single player's state with realistic ship physics
func (w *World) updatePlayer(player *Player, input *InputMsg) {
	if player.State != StateAlive {
		return
	}

	// Get stat upgrade effects for movement calculations
	statEffects := GetStatUpgradeEffects(player)

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
	maxSpeed := BaseShipMaxSpeed + statEffects["moveSpeedBonus"] - (BaseShipMaxSpeed * statEffects["speedReduction"])
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
	baseTurnSpeed := BaseShipTurnSpeed + statEffects["turnSpeedBonus"]
	scaledTurnSpeed := baseTurnSpeed * turnFactor * lengthFactor

	// Handle turning (A/D keys) and track angular velocity
	var angularChange float32 = 0
	if input.Left {
		angularChange = -scaledTurnSpeed
		player.Angle -= scaledTurnSpeed
	}
	if input.Right {
		angularChange = scaledTurnSpeed
		player.Angle += scaledTurnSpeed
	}

	// Store current angular velocity for physics calculations
	player.AngularVelocity = angularChange

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
	w.fireModularUpgrades(player, now)

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
			w.sendAvailableUpgrades(client)
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

			var upgradeType UpgradeType
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
				if player.ShipConfig.ApplyUpgrade(upgradeType, input.UpgradeChoice) {
					player.AvailableUpgrades--
					client.LastUpgrade = now // Update last upgrade time
					log.Printf("Player %d applied upgrade %s:%s, remaining upgrades: %d",
						player.ID, upgradeType, input.UpgradeChoice, player.AvailableUpgrades)
					// Send updated available upgrades to client
					w.sendAvailableUpgrades(client)
				}
			}
		}

		// Clear upgrade input to prevent multiple upgrades per frame
		input.SelectUpgrade = ""
		input.UpgradeChoice = ""
	}

	// Handle stat upgrade purchases
	if input.StatUpgradeType != "" {
		statUpgradeType := StatUpgradeType(input.StatUpgradeType)
		if UpgradeStatLevel(player, statUpgradeType) {
			log.Printf("Player %d upgraded %s to level %d, coins remaining: %d",
				player.ID, statUpgradeType, player.StatUpgrades[statUpgradeType].Level, player.Coins)
		}
		input.StatUpgradeType = "" // Clear input
	}

	// Handle health regeneration from auto repairs upgrade
	regenRate := statEffects["healthRegen"]
	// Regenerate health based on time elapsed
	elapsedSeconds := float32(now.Sub(player.LastRegenTime).Seconds())
	if elapsedSeconds >= 0.2 {
		healthToRegen := int(elapsedSeconds * regenRate)
		if healthToRegen > 0 && player.Health < player.MaxHealth {
			fmt.Println("Health to regen:", healthToRegen, "Elapsed seconds:", elapsedSeconds, "Regen rate:", regenRate) // --- IGNORE ---w
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

// checkCollisions handles player-item collisions
func (w *World) checkCollisions() {
	for playerID, player := range w.players {
		if player.State != StateAlive {
			continue
		}

		// Check item collisions using rectangular bounding boxes
		for itemID, item := range w.items {
			if w.checkPlayerItemCollision(player, item) {
				w.collectItem(playerID, itemID)
			}
		}
	}
}

// collectItem handles when a player collects an item
func (w *World) collectItem(playerID, itemID uint32) {
	player := w.players[playerID]
	item := w.items[itemID]

	// Use the mechanics system to apply item effects
	w.mechanics.ApplyItemEffect(player, item)

	delete(w.items, itemID)
}

// spawnPlayer spawns a player at a random safe location
func (w *World) spawnPlayer(player *Player) {
	// Simple random spawn - could be improved to avoid other players
	player.X = float32(rand.Intn(int(WorldWidth-100)) + 50)
	player.Y = float32(rand.Intn(int(WorldHeight-100)) + 50)
	player.State = StateAlive
}

// handleRespawns checks for dead players that need to respawn
func (w *World) handleRespawns() {
	now := time.Now()
	for _, player := range w.players {
		if player.State == StateDead && now.After(player.RespawnTime) {
			// Respawn the player
			player.Experience = 0
			player.Coins = 0
			player.Level = 1
			player.Health = player.MaxHealth
			player.State = StateAlive
			player.LastRegenTime = now       // Reset health regen timer for respawned player
			player.LastCollisionDamage = now // Reset collision damage timer for respawned player
			w.spawnPlayer(player)
			log.Printf("Player %d (%s) respawned", player.ID, player.Name)
		}
	}
}

// spawnItems continuously spawns items in the world
func (w *World) spawnItems() {
	foodTicker := time.NewTicker(time.Second * 1)    // Spawn food every 1 second
	specialTicker := time.NewTicker(time.Second * 5) // Spawn special items every 5 seconds
	defer foodTicker.Stop()
	defer specialTicker.Stop()

	for w.running {
		select {
		case <-foodTicker.C:
			w.mu.Lock()
			if len(w.items) < 100 { // Max 100 items at once
				w.mechanics.SpawnFoodItems()
			}
			w.mu.Unlock()
		}
	}
}

// broadcastSnapshot sends the current game state to all clients
func (w *World) broadcastSnapshot() {
	snapshot := Snapshot{
		Type:    MsgTypeSnapshot,
		Players: make([]Player, 0, len(w.players)),
		Items:   make([]GameItem, 0, len(w.items)),
		Bullets: make([]Bullet, 0, len(w.bullets)),
		Time:    time.Now().UnixMilli(),
	}

	// Add all players to snapshot
	for _, player := range w.players {
		snapshot.Players = append(snapshot.Players, *player)
	}

	// Add all items to snapshot
	for _, item := range w.items {
		snapshot.Items = append(snapshot.Items, *item)
	}

	// Add all bullets to snapshot
	for _, bullet := range w.bullets {
		snapshot.Bullets = append(snapshot.Bullets, *bullet)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		log.Printf("Error marshaling snapshot: %v", err)
		return
	}

	// Send to all clients
	for _, client := range w.clients {
		select {
		case client.Send <- data:
		default:
			// Channel full, skip this client
		}
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

// sendAvailableUpgrades sends available upgrades to a specific client
func (w *World) sendAvailableUpgrades(client *Client) {
	upgrades := make(map[string][]UpgradeInfo)

	// Get available upgrades for each type and convert to simplified format
	upgradeTypes := []UpgradeType{UpgradeTypeSide, UpgradeTypeTop, UpgradeTypeFront, UpgradeTypeRear}

	for _, upgradeType := range upgradeTypes {
		availableUpgrades := client.Player.ShipConfig.GetAvailableUpgrades(upgradeType)
		upgradeInfos := make([]UpgradeInfo, 0, len(availableUpgrades))

		for _, upgrade := range availableUpgrades {
			if upgrade != nil {
				upgradeInfos = append(upgradeInfos, UpgradeInfo{
					Name: upgrade.Name,
					Type: string(upgrade.Type),
				})
			}
		}

		upgrades[string(upgradeType)] = upgradeInfos
	}

	upgradesMsg := AvailableUpgradesMsg{
		Type:     "availableUpgrades",
		Upgrades: upgrades,
	}

	data, err := json.Marshal(upgradesMsg)
	if err != nil {
		log.Printf("Error marshaling available upgrades message: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		// Channel full, skip
		log.Printf("Could not send available upgrades to client %d", client.ID)
	}
}

// HandleInput processes input from a client
func (w *World) HandleInput(clientID uint32, input InputMsg) {
	if client, exists := w.GetClient(clientID); exists {
		client.mu.Lock()
		client.Input = input
		client.LastSeen = time.Now()
		client.mu.Unlock()
	}
}

// keepPlayerInBounds ensures a player stays within the world boundaries
func (w *World) keepPlayerInBounds(player *Player) {
	player.X = float32(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldWidth-player.ShipConfig.Size/2), float64(player.X))))
	player.Y = float32(math.Max(float64(player.ShipConfig.Size/2), math.Min(float64(WorldHeight-player.ShipConfig.Size/2), float64(player.Y))))
}

// updateBullets handles bullet movement and cleanup
func (w *World) updateBullets() {
	now := time.Now()

	for id, bullet := range w.bullets {
		// Check if bullet has expired
		if now.Sub(bullet.CreatedAt).Seconds() >= BulletLifetime {
			delete(w.bullets, id)
			continue
		}

		// Update bullet position
		bullet.X += bullet.VelX
		bullet.Y += bullet.VelY

		// Remove bullets that are out of bounds
		if bullet.X < 0 || bullet.X > WorldWidth || bullet.Y < 0 || bullet.Y > WorldHeight {
			delete(w.bullets, id)
			continue
		}

		// Check collision with players
		for playerID, player := range w.players {
			// Skip if bullet owner or player is dead
			if bullet.OwnerID == playerID || player.State != StateAlive {
				continue
			}

			// Use rectangular bounding box collision similar to ship collisions
			if w.checkBulletPlayerCollision(bullet, player) {
				// Apply damage
				damage := bullet.Damage
				if damage == 0 {
					damage = BulletDamage // Fallback to default for legacy bullets
				}
				player.Health -= damage

				// Remove the bullet
				delete(w.bullets, id)

				// Check if player died
				if player.Health <= 0 {
					player.Health = 0
					player.State = StateDead
					player.RespawnTime = now.Add(time.Duration(RespawnDelay) * time.Second)
					log.Printf("Player %d (%s) was killed by Player %d", playerID, player.Name, bullet.OwnerID)

					// Handle kill rewards and victim penalties
					if shooter, exists := w.players[bullet.OwnerID]; exists {
						// Calculate rewards from victim (half their resources)
						xpReward := min(player.Experience/2, 100)
						coinReward := min(player.Coins/2, 200)

						// Cap coin reward at 2000
						if coinReward > 2000 {
							coinReward = 2000
						}

						// Award to killer
						shooter.AddExperience(xpReward)
						shooter.Score += xpReward // Also add to score for leaderboard
						shooter.Coins += coinReward

						// Victim loses half their resources
						player.Experience = player.Experience / 2
						player.Coins = player.Coins / 2

						log.Printf("Player %d gained %d XP and %d coins for killing Player %d (victim lost %d XP and %d coins)", bullet.OwnerID, xpReward, coinReward, playerID, player.Experience, player.Coins)
					}
				}

				break // Bullet hit something, stop checking other players
			}
		}
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

// updateShipDimensions updates ship dimensions based on cannon and turret count
func (w *World) updateShipDimensions(player *Player) {
	sc := &player.ShipConfig
	sc.CalculateShipDimensions()

	// Update positions for all upgrades
	sc.UpdateUpgradePositions()
}

// fireModularUpgrades fires weapons based on upgrade categories with per-category cooldowns
func (w *World) fireModularUpgrades(player *Player, now time.Time) {
	// Fire side upgrades (cannons) if input is pressed and cooldown allows

	w.fireSideUpgrade(player, now)
	w.fireTopUpgrade(player, now)
	w.fireFrontUpgrade(player, now)
	w.fireRearUpgrade(player, now)
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

	fired := false
	cannonCount := len(upgrade.Cannons) / 2 // Half are left, half are right

	// Fire left side cannons
	for i := 0; i < cannonCount; i++ {
		cannon := upgrade.Cannons[i] // Use pointer to modify original cannon
		// Calculate left side angle: ship angle + 90 degrees (π/2)
		leftAngle := player.Angle + float32(math.Pi/2)
		bullets := cannon.Fire(w, player, leftAngle, now)
		for _, bullet := range bullets {
			w.bullets[bullet.ID] = bullet
			fired = true
		}
	}

	// Fire right side cannons
	for i := cannonCount; i < len(upgrade.Cannons); i++ {
		cannon := upgrade.Cannons[i] // Use pointer to modify original cannon
		// Calculate right side angle: ship angle - 90 degrees (-π/2)
		rightAngle := player.Angle - float32(math.Pi/2)
		bullets := cannon.Fire(w, player, rightAngle, now)
		for _, bullet := range bullets {
			w.bullets[bullet.ID] = bullet
			fired = true
		}
	}

	return fired
}

// fireTopUpgrade fires top-mounted turrets from the single top upgrade
func (w *World) fireTopUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.TopUpgrade == nil || player.ShipConfig.TopUpgrade.Type != UpgradeTypeTop {
		return false
	}

	upgrade := player.ShipConfig.TopUpgrade
	fired := false

	// Fire all turrets in the upgrade simultaneously
	for i := range upgrade.Turrets {
		turret := upgrade.Turrets[i] // Already a pointer to maintain state changes
		bullets := turret.Fire(w, player, now)

		if len(bullets) > 0 {
			for _, bullet := range bullets {
				w.bullets[bullet.ID] = bullet
			}
			fired = true
		}
	}

	return fired
}

// fireFrontUpgrade fires front-mounted weapons from the single front upgrade
func (w *World) fireFrontUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.FrontUpgrade == nil || player.ShipConfig.FrontUpgrade.Type != UpgradeTypeFront {
		return false
	}

	upgrade := player.ShipConfig.FrontUpgrade
	fired := false

	// Fire all cannons in the upgrade simultaneously
	for i := range upgrade.Cannons {
		cannon := upgrade.Cannons[i] // Use pointer to modify original cannon
		bullets := cannon.Fire(w, player, cannon.Angle, now)
		for _, bullet := range bullets {
			w.bullets[bullet.ID] = bullet
			fired = true
		}
	}

	// Fire all turrets in the upgrade simultaneously
	for i := range upgrade.Turrets {
		turret := upgrade.Turrets[i]
		bullets := turret.Fire(w, player, now)

		if len(bullets) > 0 {
			for _, bullet := range bullets {
				w.bullets[bullet.ID] = bullet
			}
			fired = true
		}
	}

	return fired
}

// fireRearUpgrade fires rear-mounted weapons from the single rear upgrade
func (w *World) fireRearUpgrade(player *Player, now time.Time) bool {
	if player.ShipConfig.RearUpgrade == nil || player.ShipConfig.RearUpgrade.Type != UpgradeTypeRear {
		return false
	}

	upgrade := player.ShipConfig.RearUpgrade
	fired := false

	// Fire all cannons in the upgrade simultaneously
	for i := range upgrade.Cannons {
		cannon := upgrade.Cannons[i] // Use pointer to modify original cannon
		bullets := cannon.Fire(w, player, cannon.Angle, now)
		for _, bullet := range bullets {
			w.bullets[bullet.ID] = bullet
			fired = true
		}
	}

	// Fire all turrets in the upgrade simultaneously
	for i := range upgrade.Turrets {
		turret := upgrade.Turrets[i]
		bullets := turret.Fire(w, player, now)

		if len(bullets) > 0 {
			for _, bullet := range bullets {
				w.bullets[bullet.ID] = bullet
			}
			fired = true
		}
	}

	return fired
}

// updateModularTurretAiming updates turret aiming using the new modular system
func (w *World) updateModularTurretAiming(player *Player, input *InputMsg) {
	mouseWorldX := input.Mouse.X
	mouseWorldY := input.Mouse.Y

	// Update turrets in all upgrade categories
	upgrades := []*ShipUpgrade{player.ShipConfig.TopUpgrade, player.ShipConfig.FrontUpgrade, player.ShipConfig.RearUpgrade}

	for _, upgrade := range upgrades {
		if upgrade != nil {
			for i := range upgrade.Turrets {
				turret := upgrade.Turrets[i]
				turret.UpdateAiming(player, mouseWorldX, mouseWorldY)
			}
		}
	}
}
