package game

import (
	"math"
	"time"
)

// WeaponType defines the category of weapon
type WeaponType string

const (
	WeaponTypeCannon  WeaponType = "cannon"
	WeaponTypeTurret  WeaponType = "turret"
	WeaponTypeRam     WeaponType = "ram"
	WeaponTypeRudder  WeaponType = "rudder"
	WeaponTypeScatter WeaponType = "scatter"
)

// CannonStats holds the properties of a cannon
type CannonStats struct {
	ReloadTime      float32 // Seconds between shots
	BulletSpeedMod  float32 // Multiplier for bullet speed (1.0 = normal)
	BulletDamageMod float32 // Multiplier for bullet damage (1.0 = normal)
	BulletCount     int     // Number of bullets fired per shot (for scatter cannons)
	SpreadAngle     float32 // Spread angle for multiple bullets (radians)
	Range           float32 // Maximum effective range (0 = unlimited)
	Size            float32 // Visual size of the cannon
}

// Cannon represents a basic weapon that fires bullets
type Cannon struct {
	ID           uint32      `json:"id"`
	Position     Position    `json:"position"` // Relative position from ship center
	Angle        float32     `json:"angle"`    // Fixed firing angle relative to ship
	Stats        CannonStats `json:"stats"`
	LastFireTime time.Time   `json:"-"`
	Type         WeaponType  `json:"type"`
}

// CanFire checks if the cannon is ready to fire based on reload time
func (c *Cannon) CanFire(now time.Time) bool {
	return now.Sub(c.LastFireTime).Seconds() >= float64(c.Stats.ReloadTime)
}

// Fire creates bullets from this cannon
func (c *Cannon) Fire(world *World, player *Player, targetAngle float32, now time.Time) []*Bullet {
	if !c.CanFire(now) {
		return nil
	}

	bullets := make([]*Bullet, 0, c.Stats.BulletCount)

	// Calculate world position of cannon
	cos := float32(math.Cos(float64(player.Angle)))
	sin := float32(math.Sin(float64(player.Angle)))
	worldX := player.X + (c.Position.X*cos - c.Position.Y*sin)
	worldY := player.Y + (c.Position.X*sin + c.Position.Y*cos)

	// Create bullets
	for i := 0; i < c.Stats.BulletCount; i++ {
		// Calculate bullet angle (with spread for multi-bullet cannons)
		bulletAngle := targetAngle
		if c.Stats.BulletCount > 1 {
			// Distribute bullets evenly across spread angle
			spreadOffset := c.Stats.SpreadAngle * (float32(i)/float32(c.Stats.BulletCount-1) - 0.5)
			bulletAngle += spreadOffset
		}

		// Base bullet velocity
		bulletSpeed := BulletSpeed * c.Stats.BulletSpeedMod
		bulletVelX := float32(math.Cos(float64(bulletAngle))) * bulletSpeed
		bulletVelY := float32(math.Sin(float64(bulletAngle))) * bulletSpeed

		// Add ship's linear velocity
		bulletVelX += player.VelX * 0.7
		bulletVelY += player.VelY * 0.7

		// Add tangential velocity from ship rotation
		if player.AngularVelocity != 0 {
			tangentialVelX := -player.AngularVelocity * c.Position.Y
			tangentialVelY := player.AngularVelocity * c.Position.X
			bulletVelX += tangentialVelX
			bulletVelY += tangentialVelY
		}

		bullet := &Bullet{
			ID:        world.bulletID,
			X:         worldX,
			Y:         worldY,
			VelX:      bulletVelX,
			VelY:      bulletVelY,
			OwnerID:   player.ID,
			CreatedAt: now,
			Size:      BulletSize,
			Damage:    int(float32(BulletDamage) * c.Stats.BulletDamageMod),
		}

		bullets = append(bullets, bullet)
		world.bulletID++
	}

	c.LastFireTime = now
	return bullets
}

// Turret represents a rotatable weapon system with one or more cannons
type Turret struct {
	ID           uint32     `json:"id"`
	Angle        float32    `json:"angle"` // Current aiming angle in world space
	Cannons      []Cannon   `json:"cannons"`
	Position     Position   `json:"position"`  // Relative position from ship center
	TurnSpeed    float32    `json:"turnSpeed"` // How fast turret can rotate (rad/s)
	LastFireTime time.Time  `json:"-"`
	Type         WeaponType `json:"type"`
}

// UpdateAiming updates the turret's angle to aim at target position
func (t *Turret) UpdateAiming(player *Player, targetX, targetY float32) {
	// Calculate turret world position
	cos := float32(math.Cos(float64(player.Angle)))
	sin := float32(math.Sin(float64(player.Angle)))
	turretWorldX := player.X + (t.Position.X*cos - t.Position.Y*sin)
	turretWorldY := player.Y + (t.Position.X*sin + t.Position.Y*cos)

	// Calculate desired angle to target
	dx := targetX - turretWorldX
	dy := targetY - turretWorldY
	targetAngle := float32(math.Atan2(float64(dy), float64(dx)))

	// For now, instantly snap to target (can add smooth rotation later)
	t.Angle = targetAngle
}

// CanFire checks if any cannon in the turret can fire and target is in range
func (t *Turret) CanFire(player *Player, targetX, targetY float32, now time.Time) bool {
	// Check if any cannon can fire
	for _, cannon := range t.Cannons {
		if cannon.CanFire(now) {
			return true
		}
	}
	return false
}

// Fire makes all cannons in the turret fire simultaneously (ignore individual reload times)
func (t *Turret) Fire(world *World, player *Player, targetX, targetY float32, now time.Time) []*Bullet {
	// Check range only (ignore individual cannon reload times for simultaneous firing)
	var allBullets []*Bullet

	// Fire ALL cannons simultaneously (ignore individual reload times)
	for i := range t.Cannons {
		cannon := &t.Cannons[i]
		bullets := cannon.Fire(world, player, t.Angle, now)
		allBullets = append(allBullets, bullets...)
	}

	if len(allBullets) > 0 {
		t.LastFireTime = now
	}

	return allBullets
}

// Predefined cannon types for easy configuration
func NewBasicCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1.0, // 1 second reload
		BulletSpeedMod:  1.0, // Normal speed
		BulletDamageMod: 1.0, // Normal damage
		BulletCount:     1,   // Single shot
		SpreadAngle:     0,   // No spread
		Range:           0,   // Unlimited range
		Size:            1.0, // Normal size
	}
}

func NewFastCannon() CannonStats {
	return CannonStats{
		ReloadTime:      0.5, // Fast reload
		BulletSpeedMod:  1.2, // 20% faster bullets
		BulletDamageMod: 0.8, // 20% less damage
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            0.8,
	}
}

func NewHeavyCannon() CannonStats {
	return CannonStats{
		ReloadTime:      2.0, // Slow reload
		BulletSpeedMod:  0.8, // 20% slower bullets
		BulletDamageMod: 1.5, // 50% more damage
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            1.5,
	}
}

func NewScatterCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1.5,
		BulletSpeedMod:  0.9,
		BulletDamageMod: 0.6,
		BulletCount:     3,   // Fires 3 bullets
		SpreadAngle:     0.5, // ~30 degree spread
		Range:           300, // Limited range
		Size:            1.2,
	}
}

func NewTurretCannon() CannonStats {
	return CannonStats{
		ReloadTime:      0.5, // Fast turret firing
		BulletSpeedMod:  1.0,
		BulletDamageMod: 1.0,
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            0.9,
	}
}
