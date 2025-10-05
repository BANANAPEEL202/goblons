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

// HandlePlayerCollisions checks and handles collisions between players
func (gm *GameMechanics) HandlePlayerCollisions() {
	players := make([]*Player, 0, len(gm.world.players))
	for _, player := range gm.world.players {
		if player.State == StateAlive {
			players = append(players, player)
		}
	}

	// Check player vs player collisions
	for i := 0; i < len(players); i++ {
		for j := i + 1; j < len(players); j++ {
			player1 := players[i]
			player2 := players[j]

			distance := gm.calculateDistance(player1.X, player1.Y, player2.X, player2.Y)
			minDistance := (player1.Size + player2.Size) / 2

			if distance < minDistance {
				gm.handlePlayerCollision(player1, player2)
			}
		}
	}
}

// handlePlayerCollision handles what happens when two players collide
func (gm *GameMechanics) handlePlayerCollision(player1, player2 *Player) {
	// In doblons.io style, larger players can absorb smaller ones
	sizeDifference := math.Abs(float64(player1.Size - player2.Size))

	if sizeDifference > 5 { // Minimum size difference for absorption
		var winner, loser *Player
		if player1.Size > player2.Size {
			winner, loser = player1, player2
		} else {
			winner, loser = player2, player1
		}

		// Transfer some size and score
		sizeGain := loser.Size * 0.3                 // Winner gets 30% of loser's size
		scoreGain := int(float32(loser.Score) * 0.5) // Winner gets 50% of loser's score

		winner.Size += sizeGain
		winner.Score += scoreGain

		// Respawn the loser
		gm.respawnPlayer(loser)

	} else {
		// Players are similar size, just push them apart
		gm.separatePlayers(player1, player2)
	}
}

// separatePlayers pushes two overlapping players apart
func (gm *GameMechanics) separatePlayers(player1, player2 *Player) {
	dx := player1.X - player2.X
	dy := player1.Y - player2.Y
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if distance == 0 {
		// Players are at exact same position, separate randomly
		angle := rand.Float64() * 2 * math.Pi
		dx = float32(math.Cos(angle))
		dy = float32(math.Sin(angle))
		distance = 1
	}

	// Normalize and separate
	dx /= distance
	dy /= distance

	separation := (player1.Size+player2.Size)/2 - distance + 1
	moveDistance := separation / 2

	player1.X += dx * moveDistance
	player1.Y += dy * moveDistance
	player2.X -= dx * moveDistance
	player2.Y -= dy * moveDistance

	// Keep players within bounds
	gm.world.keepPlayerInBounds(player1)
	gm.world.keepPlayerInBounds(player2)
}

// respawnPlayer resets a player's state and position
func (gm *GameMechanics) respawnPlayer(player *Player) {
	player.Size = PlayerSize
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
	safeDistance := size * 3 // Safe distance is 3 times the player size

	for _, otherPlayer := range gm.world.players {
		if otherPlayer.State != StateAlive {
			continue
		}

		distance := gm.calculateDistance(x, y, otherPlayer.X, otherPlayer.Y)
		if distance < safeDistance {
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
		player.Size += 0.5
		player.Score += item.Value

	case "coin":
		player.Score += item.Value

	case "health_pack":
		player.Health = int(math.Min(float64(player.MaxHealth), float64(player.Health+item.Value)))

	case "size_boost":
		player.Size += float32(item.Value)

	case "speed_boost":
		// This would require adding temporary effects system
		player.Score += 5 // Give some score for now

	case "score_multiplier":
		player.Score = int(float32(player.Score) * float32(item.Value))
	}

	// Cap maximum size
	if player.Size > 100 {
		player.Size = 100
	}
}
