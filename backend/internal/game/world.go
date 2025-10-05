package game

import (
	"encoding/json"
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

	// Scale turn speed based on current speed
	// Example: turn faster at low speed, slower at high speed
	// The "+0.1" ensures some minimum turning ability even when stationary
	turnFactor := speed / ShipMaxSpeed

	scaledTurnSpeed := ShipTurnSpeed * turnFactor

	// Handle turning (A/D keys)
	if input.Left {

		player.Angle -= scaledTurnSpeed
	}
	if input.Right {
		player.Angle += scaledTurnSpeed
	}

	// Handle shooting (left and right cannons)
	now := time.Now()
	if (input.ShootLeft || input.ShootRight) && now.Sub(player.LastShotTime).Seconds() >= CannonCooldown {
		w.fireCannon(player, true)  // Left cannon
		w.fireCannon(player, false) // Right cannon
		player.LastShotTime = now
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
			if distance < player.Size/2+10 { // Item pickup radius
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

// fireCannon creates a bullet from the specified cannon (left=true, right=false)
func (w *World) fireCannon(player *Player, isLeftCannon bool) {
	// Calculate cannon position on the side of the ship
	// Cannons are positioned perpendicular to the ship's facing direction
	sideAngle := player.Angle + float32(math.Pi/2) // 90 degrees to the left of ship's facing direction
	if !isLeftCannon {
		sideAngle = player.Angle - float32(math.Pi/2) // 90 degrees to the right of ship's facing direction
	}

	// Position cannon on the side of the ship
	cannonX := player.X + float32(math.Cos(float64(sideAngle)))*CannonDistance
	cannonY := player.Y + float32(math.Sin(float64(sideAngle)))*CannonDistance

	// Bullet fires perpendicular to the ship (in the same direction as the cannon positioning)
	bulletVelX := float32(math.Cos(float64(sideAngle))) * BulletSpeed
	bulletVelY := float32(math.Sin(float64(sideAngle))) * BulletSpeed

	// Create bullet
	bullet := &Bullet{
		ID:        w.bulletID,
		X:         cannonX,
		Y:         cannonY,
		VelX:      bulletVelX,
		VelY:      bulletVelY,
		OwnerID:   player.ID,
		CreatedAt: time.Now(),
		Size:      BulletSize,
	}

	w.bullets[w.bulletID] = bullet
	w.bulletID++
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

		// TODO: Add collision detection with players here
	}
}
