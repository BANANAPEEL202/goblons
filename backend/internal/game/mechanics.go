package game

import (
	"math"
	"math/rand"
	"time"
)

// GameMechanics handles specific game logic like combat, collecting, etc.
type GameMechanics struct {
	world *World
}

// isFrontalRam returns true if attacker is ramming the victim frontally
func (gm *GameMechanics) isFrontalRam(attacker, victim *Player) bool {
	// Calculate vector from attacker to victim
	dx := victim.X - attacker.X
	dy := victim.Y - attacker.Y
	angleToVictim := math.Atan2(float64(dy), float64(dx))
	// Attacker's facing angle
	attackerAngle := float64(attacker.Angle)
	// Calculate the shortest angular distance between the two angles
	angleDiff := math.Abs(angleToVictim - attackerAngle)
	// Handle wraparound case (e.g., 350° vs 10° should be 20°, not 340°)
	if angleDiff > math.Pi {
		angleDiff = 2*math.Pi - angleDiff
	}
	// Consider frontal if within 45 degrees (pi/4)
	return angleDiff < math.Pi/4
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
	bbox1 := player1.GetShipBoundingBox()
	bbox2 := player2.GetShipBoundingBox()

	// Check if bounding boxes overlap
	return bbox1.MinX < bbox2.MaxX && bbox1.MaxX > bbox2.MinX &&
		bbox1.MinY < bbox2.MaxY && bbox1.MaxY > bbox2.MinY
}

// BoundingBox represents a rectangular bounding box
type BoundingBox struct {
	MinX, MinY, MaxX, MaxY float64
}

// handlePlayerCollision handles what happens when two players collide
func (gm *GameMechanics) handlePlayerCollision(player1, player2 *Player) {
	now := time.Now()

	// Ships push against each other when they collide
	gm.pushShipsApart(player1, player2)

	// Apply collision damage if enough time has passed since last collision damage
	gm.applyCollisionDamage(player1, player2, now)

	// Frontal ram logic
	if gm.isFrontalRam(player1, player2) && player1.ShipConfig.FrontUpgrade != nil && player1.ShipConfig.FrontUpgrade.Name == "Ram" {
		ramDamage := 15 // Base ram damage, can be made configurable/stat-based
		gm.ApplyDamage(player2, ramDamage, player1, KillCauseRam, now)
	}
	if gm.isFrontalRam(player2, player1) && player2.ShipConfig.FrontUpgrade != nil && player2.ShipConfig.FrontUpgrade.Name == "Ram" {
		ramDamage := 1
		gm.ApplyDamage(player1, ramDamage, player2, KillCauseRam, now)
	}
}

// pushShipsApart pushes two colliding ships apart based on their bounding boxes
func (gm *GameMechanics) pushShipsApart(p1, p2 *Player) {
	bbox1 := p1.GetShipBoundingBox()
	bbox2 := p2.GetShipBoundingBox()

	// Calculate overlap in both axes
	overlapX := float64(math.Min(float64(bbox1.MaxX), float64(bbox2.MaxX))) - float64(math.Max(float64(bbox1.MinX), float64(bbox2.MinX)))
	overlapY := float64(math.Min(float64(bbox1.MaxY), float64(bbox2.MaxY))) - float64(math.Max(float64(bbox1.MinY), float64(bbox2.MinY)))

	// Only push if there's actual overlap
	if overlapX > 0 && overlapY > 0 {
		// Calculate center-to-center distance for push direction
		dx := p1.X - p2.X
		dy := p1.Y - p2.Y
		distance := float64(math.Sqrt(float64(dx*dx + dy*dy)))

		// Handle case where ships are at same position
		if distance == 0 {
			angle := rand.Float64() * 2 * math.Pi
			dx = float64(math.Cos(angle))
			dy = float64(math.Sin(angle))
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
			restitution := float64(0.5)
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
			restitution := float64(0.5)
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

// applyCollisionDamage handles collision damage between two players
func (gm *GameMechanics) applyCollisionDamage(player1, player2 *Player, now time.Time) {
	cooldown := time.Duration(CollisionCooldown * float64(time.Second))

	// Check if enough time has passed since last collision damage for player1
	if now.Sub(player1.LastCollisionDamage) >= cooldown {
		// Calculate damage from player1 to player2
		damageToPlayer2 := BaseCollisionDamage + int(player1.Modifiers.BodyDamageBonus)
		gm.ApplyDamage(player2, damageToPlayer2, player1, KillCauseCollision, now)

		player1.LastCollisionDamage = now
	}

	// Check if enough time has passed since last collision damage for player2
	if now.Sub(player2.LastCollisionDamage) >= cooldown {
		// Calculate damage from player2 to player1
		damageToPlayer1 := BaseCollisionDamage + int(player2.Modifiers.BodyDamageBonus)
		gm.ApplyDamage(player1, damageToPlayer1, player2, KillCauseCollision, now)

		player2.LastCollisionDamage = now
	}
}

// SpawnFoodItems spawns the new 4-tier item system around the map
func (gm *GameMechanics) SpawnFoodItems() {
	// Define the 4 item types with their properties
	itemTypes := []struct {
		name   string
		coins  int
		xp     int
		weight int // Spawn weight (higher = more common)
	}{
		{ItemTypeGrayCircle, 10, 10, 30},   // Most common
		{ItemTypeYellowCircle, 10, 10, 20}, // Common
		{ItemTypeOrangeCircle, 20, 20, 20}, // Uncommon
		{ItemTypeBlueDiamond, 30, 30, 10},  // Rare
	}

	// Calculate total weight
	totalWeight := 0
	for _, itemType := range itemTypes {
		totalWeight += itemType.weight
	}

	// Spawn until we reach the maximum item count
	for len(gm.world.items) < MaxItems {
		// Select item type based on weighted probability
		roll := rand.Intn(totalWeight)
		currentWeight := 0
		selectedType := itemTypes[0] // fallback

		for _, itemType := range itemTypes {
			currentWeight += itemType.weight
			if roll < currentWeight {
				selectedType = itemType
				break
			}
		}

		itemID := gm.world.itemID
		gm.world.itemID++

		item := &GameItem{
			ID:    itemID,
			X:     float64(rand.Intn(int(WorldWidth-50)) + 25),
			Y:     float64(rand.Intn(int(WorldHeight-50)) + 25),
			Type:  selectedType.name,
			Coins: selectedType.coins,
			XP:    selectedType.xp,
		}
		gm.world.items[item.ID] = item
	}
}
