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

	// Initialize ship dimensions and cannon positions
	w.updateShipDimensions(client.Player)
	w.updatePlayerCannonPositions(client.Player)

	// Send welcome message to the new client with their player ID
	w.sendWelcomeMessage(client)

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
	w.mu.RLock()
	defer w.mu.RUnlock()
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

	speed := min(float32(math.Sqrt(float64(player.VelX*player.VelX+player.VelY*player.VelY))), ShipMaxSpeed)

	// Scale turn speed based on current speed and ship length
	// Example: turn faster at low speed, slower at high speed
	// Longer ships turn slower (more realistic naval physics)
	turnFactor := speed / ShipMaxSpeed

	// Calculate length factor - longer ships turn slower
	// Base length for comparison (1 cannon = standard ship)
	baseShipLength := float32(PlayerSize * 1.2)        // 1 cannon ship has no length multiplier
	lengthFactor := baseShipLength / player.ShipLength // Longer ships get smaller factor

	scaledTurnSpeed := ShipTurnSpeed * turnFactor * lengthFactor

	// Handle turning (A/D keys)
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
	if newSpeed > ShipMaxSpeed {
		speedRatio := ShipMaxSpeed / newSpeed
		player.VelX *= speedRatio
		player.VelY *= speedRatio
	}

	// Update position
	player.X += player.VelX
	player.Y += player.VelY

	// Handle shooting (left and right cannons)
	now := time.Now()
	if (input.ShootLeft || input.ShootRight) && now.Sub(player.LastShotTime).Seconds() >= CannonCooldown {
		w.fireAllCannons(player, true)  // Left side cannons
		w.fireAllCannons(player, false) // Right side cannons
		player.LastShotTime = now
	}

	// Handle ship upgrades
	if input.UpgradeCannons {
		w.UpgradePlayerCannons(player.ID)
	}
	if input.DowngradeCannons {
		w.DowngradePlayerCannons(player.ID)
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

		// Check item collisions
		for itemID, item := range w.items {
			distance := float32(math.Sqrt(float64((player.X-item.X)*(player.X-item.X) + (player.Y-item.Y)*(player.Y-item.Y))))
			if distance < player.CollisionRadius+10 { // Item pickup radius using dynamic collision radius
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
			player.Health = player.MaxHealth
			player.State = StateAlive
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
		case <-specialTicker.C:
			w.mu.Lock()
			if len(w.items) < 100 {
				w.mechanics.SpawnSpecialItems()
			}
			w.mu.Unlock()
		}
	}
}

// spawnRandomItem spawns a random item at a random location
func (w *World) spawnRandomItem() {
	item := &GameItem{
		ID: w.itemID,
		X:  float32(rand.Intn(int(WorldWidth-100)) + 50),
		Y:  float32(rand.Intn(int(WorldHeight-100)) + 50),
	}
	w.itemID++

	// Random item type
	itemTypes := []struct {
		name  string
		value int
	}{
		{"coin", 10},
		{"coin", 25},
		{"health", 20},
		{"size", 1},
	}

	chosen := itemTypes[rand.Intn(len(itemTypes))]
	item.Type = chosen.name
	item.Value = chosen.value

	w.items[item.ID] = item
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
	player.X = float32(math.Max(float64(player.Size/2), math.Min(float64(WorldWidth-player.Size/2), float64(player.X))))
	player.Y = float32(math.Max(float64(player.Size/2), math.Min(float64(WorldHeight-player.Size/2), float64(player.Y))))
}

// fireAllCannons fires all cannons on the specified side of the ship
func (w *World) fireAllCannons(player *Player, isLeftSide bool) {
	// Calculate the side angle (perpendicular to ship's facing direction)
	sideAngle := player.Angle + float32(math.Pi/2) // 90 degrees to the left of ship's facing direction
	if !isLeftSide {
		sideAngle = player.Angle - float32(math.Pi/2) // 90 degrees to the right of ship's facing direction
	}

	// Use pre-calculated cannon positions from player struct
	var cannonPositions []CannonPosition
	if isLeftSide {
		cannonPositions = player.LeftCannons
	} else {
		cannonPositions = player.RightCannons
	}

	// Transform relative positions to world coordinates and fire each cannon
	cos := float32(math.Cos(float64(player.Angle)))
	sin := float32(math.Sin(float64(player.Angle)))

	for _, cannonPos := range cannonPositions {
		// Transform relative position to world coordinates using ship's rotation
		worldX := player.X + (cannonPos.X*cos - cannonPos.Y*sin)
		worldY := player.Y + (cannonPos.X*sin + cannonPos.Y*cos)

		// Bullet fires perpendicular to the ship (in the same direction as the cannon positioning)
		// includes player velocity for more realistic shooting
		bulletVelX := float32(math.Cos(float64(sideAngle)))*BulletSpeed + player.VelX*0.7
		bulletVelY := float32(math.Sin(float64(sideAngle)))*BulletSpeed + player.VelY*0.7

		// Create bullet at world coordinates
		bullet := &Bullet{
			ID:        w.bulletID,
			X:         worldX,
			Y:         worldY,
			VelX:      bulletVelX,
			VelY:      bulletVelY,
			OwnerID:   player.ID,
			CreatedAt: time.Now(),
			Size:      BulletSize,
		}

		w.bullets[w.bulletID] = bullet
		w.bulletID++
	}
}

// calculateCannonPositions calculates relative positions for all cannons on one side of the ship
func (w *World) calculateCannonPositions(player *Player, isLeftSide bool) []CannonPosition {
	positions := make([]CannonPosition, 0, player.CannonCount)

	// Use the shaft length directly from player (already calculated in updateShipDimensions)
	shaftLength := player.ShipLength

	gunSpacing := shaftLength / float32(player.CannonCount+1)
	gunLength := player.Size * 0.35
	gunWidth := player.Size * 0.2
	shaftWidth := player.ShipWidth

	for i := 0; i < player.CannonCount; i++ {
		// Calculate horizontal cannon center position (matching frontend exactly)
		// Frontend: x = -shaftLength / 2 + (i + 1) * gunSpacing - gunLength / 2
		// This gives the LEFT edge of the cannon rectangle (fillRect x parameter)
		// To get the center, we need to add gunLength / 2
		cannonLeftEdge := -shaftLength/2 + float32(i+1)*gunSpacing - gunLength/2
		relativeX := cannonLeftEdge + gunLength/2 // Move to horizontal center of cannon

		// Calculate cannon center Y position
		// Frontend draws rectangles with fillRect(x, y, gunLength, gunWidth)
		// where y is the TOP edge of the rectangle, so center is y + gunWidth/2
		var relativeY float32
		if isLeftSide {
			// Try swapping: Q key should fire bottom cannons (positive Y)
			// Frontend "right side": y = shaftWidth / 2 (top of rectangle)
			// Center = y + gunWidth/2 = shaftWidth/2 + gunWidth/2
			relativeY = shaftWidth/2 + gunWidth/2
		} else {
			// Try swapping: E key should fire top cannons (negative Y)
			// Frontend "left side": y = -shaftWidth / 2 - gunWidth (top of rectangle)
			// Center = y + gunWidth/2 = -shaftWidth/2 - gunWidth + gunWidth/2 = -shaftWidth/2 - gunWidth/2
			relativeY = -shaftWidth/2 - gunWidth + gunWidth/2
		}

		// Store relative positions (no transformation to world coordinates)
		positions = append(positions, CannonPosition{X: relativeX, Y: relativeY})
	}

	return positions
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

			// Calculate distance between bullet and player
			distance := float32(math.Sqrt(float64((bullet.X-player.X)*(bullet.X-player.X) + (bullet.Y-player.Y)*(bullet.Y-player.Y))))

			// Check if bullet hits player (bullet size + collision radius)
			if distance < BulletSize+player.CollisionRadius {
				// Apply damage
				player.Health -= BulletDamage

				// Remove the bullet
				delete(w.bullets, id)

				// Check if player died
				if player.Health <= 0 {
					player.Health = 0
					player.State = StateDead
					player.RespawnTime = now.Add(time.Duration(RespawnDelay) * time.Second)
					log.Printf("Player %d (%s) was killed by Player %d", playerID, player.Name, bullet.OwnerID)

					// Award score to shooter
					if shooter, exists := w.players[bullet.OwnerID]; exists {
						shooter.Score += 100
					}
				}

				break // Bullet hit something, stop checking other players
			}
		}
	}
}

// UpgradePlayerCannons increases the number of cannons on a player's ship
func (w *World) UpgradePlayerCannons(playerID uint32) bool {

	player, exists := w.players[playerID]
	if !exists {
		return false
	}

	if player.CannonCount >= MaxCannonsPerSide {
		return false // Already at maximum
	}

	player.CannonCount++
	w.updateShipDimensions(player)
	w.updatePlayerCannonPositions(player) // Update cannon positions after changing count
	return true
}

// DowngradePlayerCannons decreases the number of cannons on a player's ship
func (w *World) DowngradePlayerCannons(playerID uint32) bool {
	fmt.Println("Downgrading cannons for player", playerID)
	player, exists := w.players[playerID]
	if !exists {
		return false
	}

	if player.CannonCount <= MinCannonsPerSide {
		return false // Already at minimum
	}

	player.CannonCount--
	w.updateShipDimensions(player)
	w.updatePlayerCannonPositions(player) // Update cannon positions after changing count
	return true
}

// updateShipDimensions updates ship dimensions based on cannon count
func (w *World) updateShipDimensions(player *Player) {
	// Base dimensions
	baseShaftLength := float32(PlayerSize*1.2) * 0.5 // Base shaft length (what frontend uses)
	baseWidth := float32(PlayerSize * 0.8)

	// Calculate extra length for additional cannons (same logic as frontend used to have)
	var extraShaftLength float32
	if player.CannonCount > 1 {
		gunLength := player.Size * 0.35
		spacing := gunLength * 1.5
		extraShaftLength = spacing * float32(player.CannonCount-1)
	}

	// ShipLength now represents the shaft length that frontend expects
	player.ShipLength = baseShaftLength + extraShaftLength
	player.ShipWidth = baseWidth      // Width stays constant
	player.Size = float32(PlayerSize) // Overall size stays constant for rendering
	player.CollisionRadius = calculateCollisionRadius(player.ShipLength, player.ShipWidth)
}

// updatePlayerCannonPositions calculates and stores cannon positions for a player
func (w *World) updatePlayerCannonPositions(player *Player) {
	player.LeftCannons = w.calculateCannonPositions(player, true)
	player.RightCannons = w.calculateCannonPositions(player, false)
}

// SetPlayerCannonCount sets the exact number of cannons for a player (for testing/admin)
func (w *World) SetPlayerCannonCount(playerID uint32, cannonCount int) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	player, exists := w.players[playerID]
	if !exists {
		return false
	}

	if cannonCount < MinCannonsPerSide || cannonCount > MaxCannonsPerSide {
		return false
	}

	player.CannonCount = cannonCount
	w.updateShipDimensions(player)
	w.updatePlayerCannonPositions(player) // Update cannon positions after setting count
	return true
}
