package game

import (
	"math"
	"math/rand"
)

// GameMechanics handles specific game logic like combat, collecting, etc.
type GameMechanics struct {
	world *World
}

// NewGameMechanics creates a new game mechanics handler
func NewGameMechanics(world *World) *GameMechanics {
	return &GameMechanics{world: world}
}

// HandlePlayerCollisions checks and handles collisions between players using rectangular bounding boxes
func (gm *GameMechanics) HandlePlayerCollisions() {
	players := make([]*Player, 0, len(gm.world.players))
	for _, player := range gm.world.players {
		if player.State == StateAlive {
			players = append(players, player)
		}
	}

	// Check player vs player collisions using rectangular bounding boxes
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			player1 := players[i]
			player2 := players[j]

			if gm.checkRectangularCollision(player1, player2) {
				gm.handlePlayerCollision(player1, player2)
			}
		}
	}
}

// checkRectangularCollision checks if two ships' rectangular bounding boxes collide
func (gm *GameMechanics) checkRectangularCollision(player1, player2 *Player) bool {
	bbox1 := gm.getShipBoundingBox(player1)
	bbox2 := gm.getShipBoundingBox(player2)

	// Check if bounding boxes overlap
	return bbox1.MinX < bbox2.MaxX && bbox1.MaxX > bbox2.MinX &&
		bbox1.MinY < bbox2.MaxY && bbox1.MaxY > bbox2.MinY
}

// BoundingBox represents a rectangular bounding box
type BoundingBox struct {
	MinX, MinY, MaxX, MaxY float32
}

// getShipBoundingBox calculates the axis-aligned bounding box for a rotated ship
func (gm *GameMechanics) getShipBoundingBox(player *Player) BoundingBox {
	// Calculate the four corners of the rotated ship rectangle
	halfLength := player.ShipConfig.ShipLength / 2
	halfWidth := player.ShipConfig.ShipWidth / 2

	cos := float32(math.Cos(float64(player.Angle)))
	sin := float32(math.Sin(float64(player.Angle)))

	// Local corners (relative to ship center)
	corners := []struct{ x, y float32 }{
		{-halfLength, -halfWidth}, // Back-left
		{halfLength, -halfWidth},  // Front-left
		{halfLength, halfWidth},   // Front-right
		{-halfLength, halfWidth},  // Back-right
	}

	// Transform corners to world coordinates and find bounding box
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))

	for _, corner := range corners {
		// Rotate corner and translate to world position
		worldX := player.X + (corner.x*cos - corner.y*sin)
		worldY := player.Y + (corner.x*sin + corner.y*cos)

		if worldX < minX {
			minX = worldX
		}
		if worldX > maxX {
			maxX = worldX
		}
		if worldY < minY {
			minY = worldY
		}
		if worldY > maxY {
			maxY = worldY
		}
	}

	return BoundingBox{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
}

// handlePlayerCollision handles what happens when two players collide
func (gm *GameMechanics) handlePlayerCollision(player1, player2 *Player) {
	// Ships push against each other when they collide
	gm.pushShipsApart(player1, player2)
}

// pushShipsApart pushes two colliding ships apart based on their bounding boxes
func (gm *GameMechanics) pushShipsApart(p1, p2 *Player) {
	bbox1 := gm.getShipBoundingBox(p1)
	bbox2 := gm.getShipBoundingBox(p2)

	// Calculate overlap in both axes
	overlapX := float32(math.Min(float64(bbox1.MaxX), float64(bbox2.MaxX))) - float32(math.Max(float64(bbox1.MinX), float64(bbox2.MinX)))
	overlapY := float32(math.Min(float64(bbox1.MaxY), float64(bbox2.MaxY))) - float32(math.Max(float64(bbox1.MinY), float64(bbox2.MinY)))

	// Only push if there's actual overlap
	if overlapX > 0 && overlapY > 0 {
		// Calculate center-to-center distance for push direction
		dx := p1.X - p2.X
		dy := p1.Y - p2.Y
		distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

		// Handle case where ships are at same position
		if distance == 0 {
			angle := rand.Float64() * 2 * math.Pi
			dx = float32(math.Cos(angle))
			dy = float32(math.Sin(angle))
			distance = 1
		}

		// Normalize direction vector
		dx /= distance
		dy /= distance

		// Choose the axis with smaller overlap for more natural separation
		if overlapX < overlapY {
			// Push apart along X axis
			push := overlapX / 2
			if dx > 0 {
				p1.X += push
				p2.X -= push
			} else {
				p1.X -= push
				p2.X += push
			}

			// Apply velocity transfer
			restitution := float32(0.5)
			relVel := p1.VelX - p2.VelX
			if (dx > 0 && relVel < 0) || (dx < 0 && relVel > 0) {
				impulse := -relVel * (1 + restitution) / 2
				p1.VelX += impulse
				p2.VelX -= impulse
			}
		} else {
			// Push apart along Y axis
			push := overlapY / 2
			if dy > 0 {
				p1.Y += push
				p2.Y -= push
			} else {
				p1.Y -= push
				p2.Y += push
			}

			// Apply velocity transfer
			restitution := float32(0.5)
			relVel := p1.VelY - p2.VelY
			if (dy > 0 && relVel < 0) || (dy < 0 && relVel > 0) {
				impulse := -relVel * (1 + restitution) / 2
				p1.VelY += impulse
				p2.VelY -= impulse
			}
		}
	}

	gm.world.keepPlayerInBounds(p1)
	gm.world.keepPlayerInBounds(p2)
}

// separatePlayers pushes two overlapping players apart (legacy function for backward compatibility)
func (gm *GameMechanics) separatePlayers(player1, player2 *Player) {
	// Redirect to the new push function
	gm.pushShipsApart(player1, player2)
}

// respawnPlayer resets a player's state and position
func (gm *GameMechanics) respawnPlayer(player *Player) {
	player.ShipConfig.Size = PlayerSize
	player.Score = 0
	player.Health = player.MaxHealth
	player.State = StateSpawning

	// Find a safe spawn location
	for attempts := 0; attempts < 10; attempts++ {
		x := float32(rand.Intn(int(WorldWidth-100)) + 50)
		y := float32(rand.Intn(int(WorldHeight-100)) + 50)

		if gm.isLocationSafe(x, y, PlayerSize) {
			player.X = x
			player.Y = y
			break
		}
	}

	player.State = StateAlive
}

// isLocationSafe checks if a location is safe for spawning (no other players nearby)
func (gm *GameMechanics) isLocationSafe(x, y, size float32) bool {
	// Create a temporary player to check collision area

	// Check against all existing players
	for _, other := range gm.world.players {
		if other.State != StateAlive {
			continue
		}
		distance := gm.calculateDistance(x, y, other.X, other.Y)
		if distance < (size+other.ShipConfig.Size)/2+20 { // 20 units buffer
			return false
		}
	}

	return true
}

// calculateDistance calculates the distance between two points
func (gm *GameMechanics) calculateDistance(x1, y1, x2, y2 float32) float32 {
	dx := x1 - x2
	dy := y1 - y2
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// SpawnFoodItems spawns food items around the map
func (gm *GameMechanics) SpawnFoodItems() {
	// Spawn small food items regularly
	for i := 0; i < 5; i++ {
		item := &GameItem{
			ID:    gm.world.itemID,
			X:     float32(rand.Intn(int(WorldWidth-50)) + 25),
			Y:     float32(rand.Intn(int(WorldHeight-50)) + 25),
			Type:  "food",
			Value: 1,
		}
		gm.world.itemID++
		gm.world.items[item.ID] = item
	}
}

// SpawnSpecialItems spawns special power-up items less frequently
func (gm *GameMechanics) SpawnSpecialItems() {
	if rand.Float32() < 0.3 { // 30% chance
		itemTypes := []struct {
			name  string
			value int
		}{
			{"speed_boost", 1},
			{"size_boost", 5},
			{"health_pack", 50},
			{"score_multiplier", 2},
		}

		chosen := itemTypes[rand.Intn(len(itemTypes))]
		item := &GameItem{
			ID:    gm.world.itemID,
			X:     float32(rand.Intn(int(WorldWidth-100)) + 50),
			Y:     float32(rand.Intn(int(WorldHeight-100)) + 50),
			Type:  chosen.name,
			Value: chosen.value,
		}
		gm.world.itemID++
		gm.world.items[item.ID] = item
	}
}

// ApplyItemEffect applies the effect of a collected item to a player
func (gm *GameMechanics) ApplyItemEffect(player *Player, item *GameItem) {
	switch item.Type {
	case "food":
		player.Score += item.Value

	case "coin":
		player.Score += item.Value

	case "health_pack":
		player.Health = int(math.Min(float64(player.MaxHealth), float64(player.Health+item.Value)))

	case "speed_boost":
		// This would require adding temporary effects system
		player.Score += 5 // Give some score for now

	case "score_multiplier":
		player.Score = int(float32(player.Score) * float32(item.Value))
	}
}
