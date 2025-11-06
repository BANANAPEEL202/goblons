package game

import (
	"math"
	"time"
)

// WeaponType defines the category of weapon
type WeaponType string

const (
	WeaponTypeCannon           WeaponType = "cannon"
	WeaponTypeTurret           WeaponType = "turret"
	WeaponTypeMachineGunTurret WeaponType = "machine_gun_turret"
	WeaponTypeRam              WeaponType = "ram"
	WeaponTypeRudder           WeaponType = "rudder"
	WeaponTypeScatter          WeaponType = "scatter"
	WeaponTypeRow              WeaponType = "row"
	WeaponTypeBigTurret        WeaponType = "big_turret"
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
	RecoilTime   time.Time   `json:"recoilTime"` // When the cannon last fired (for recoil animation)
}

// CanFire checks if the cannon is ready to fire based on reload time
func (c *Cannon) CanFire(player *Player, now time.Time) bool {
	reloadTime := c.Stats.ReloadTime * player.Modifiers.ReloadSpeedMultiplier
	return float32(now.Sub(c.LastFireTime).Seconds()) >= reloadTime
}

// Fire creates bullets from this cannon
func (c *Cannon) Fire(world *World, player *Player, targetAngle float32, now time.Time) []*Bullet {
	if !c.CanFire(player, now) {
		return nil
	}
	return c.ForceFire(world, player, targetAngle, now)
}

func (c *Cannon) ForceFire(world *World, player *Player, targetAngle float32, now time.Time) []*Bullet {
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

		// Base bullet velocity with cannon range upgrade
		bulletSpeed := BulletSpeed * c.Stats.BulletSpeedMod
		bulletSpeed *= player.Modifiers.BulletSpeedMultiplier
		bulletVelX := float32(math.Cos(float64(bulletAngle))) * bulletSpeed
		bulletVelY := float32(math.Sin(float64(bulletAngle))) * bulletSpeed

		// Calculate bullet damage and size with upgrades
		baseDamage := float32(BulletDamage) * c.Stats.BulletDamageMod
		finalDamage := baseDamage * player.Modifiers.BulletDamageMultiplier // Add cannon damage bonus
		bulletSize := BulletSize * c.Stats.Size

		bullet := &Bullet{
			ID:        world.bulletID,
			X:         worldX,
			Y:         worldY,
			VelX:      bulletVelX,
			VelY:      bulletVelY,
			OwnerID:   player.ID,
			CreatedAt: now,
			Size:      bulletSize,
			Damage:    int(finalDamage),
		}

		bullets = append(bullets, bullet)
		world.bulletID++
	}

	c.LastFireTime = now
	c.RecoilTime = now
	return bullets
}

// Turret represents a rotatable weapon system with one or more cannons
type Turret struct {
	ID              uint32     `json:"id"`
	Angle           float32    `json:"angle"` // Current aiming angle in world space
	Cannons         []Cannon   `json:"cannons"`
	Position        Position   `json:"position"`  // Relative position from ship center
	TurnSpeed       float32    `json:"turnSpeed"` // How fast turret can rotate (rad/s)
	LastFireTime    time.Time  `json:"-"`
	Type            WeaponType `json:"type"`
	NextCannonIndex int        `json:"nextCannonIndex"` // For alternating fire
	RecoilTime      time.Time  `json:"recoilTime"`      // When the turret last fired (for recoil animation)
}

// UpdateAiming updates the turret's angle to aim at target position
func (t *Turret) UpdateAiming(player *Player, targetX, targetY float32) {
	// Calculate desired angle to target
	dx := targetX - player.X
	dy := targetY - player.Y
	targetAngle := float32(math.Atan2(float64(dy), float64(dx)))

	// For now, instantly snap to target (can add smooth rotation later)
	t.Angle = targetAngle
}

// Fire makes all cannons in the turret fire (simultaneously or alternating based on type)
func (t *Turret) Fire(world *World, player *Player, now time.Time) []*Bullet {
	var allBullets []*Bullet

	if t.Type == WeaponTypeMachineGunTurret && len(t.Cannons) > 1 {
		// Twin turret: fire alternating cannons with shared reload time
		if t.NextCannonIndex >= len(t.Cannons) {
			t.NextCannonIndex = 0
		}

		// Check turret reload time instead of individual cannon reload
		cannon := &t.Cannons[t.NextCannonIndex]
		reloadTime := float64(cannon.Stats.ReloadTime) * float64(player.Modifiers.ReloadSpeedMultiplier)

		if now.Sub(t.LastFireTime).Seconds() >= reloadTime {
			bullets := cannon.ForceFire(world, player, t.Angle, now)
			allBullets = append(allBullets, bullets...)

			// Move to next cannon for alternating fire
			t.NextCannonIndex = (t.NextCannonIndex + 1) % len(t.Cannons)
			t.LastFireTime = now
			t.RecoilTime = now
		}
	} else {
		// Regular turret: fire all cannons simultaneously
		for i := range t.Cannons {
			cannon := &t.Cannons[i]
			bullets := cannon.Fire(world, player, t.Angle, now)
			allBullets = append(allBullets, bullets...)
		}

		if len(allBullets) > 0 {
			t.LastFireTime = now
			t.RecoilTime = now
		}
	}

	return allBullets
}

// Predefined cannon types for easy configuration
func NewBasicCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1,   // 1 second reload
		BulletSpeedMod:  1,   // Normal speed
		BulletDamageMod: 1.0, // Normal damage
		BulletCount:     1,   // Single shot
		SpreadAngle:     0,   // No spread
		Range:           0,   // Unlimited range
		Size:            1.0, // Normal size
	}
}

func NewScatterCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1.5,
		BulletSpeedMod:  0.9,
		BulletDamageMod: 0.6,
		BulletCount:     3,   // Fires 3 bullets
		SpreadAngle:     0.5, // ~30 degree spread
		Range:           0,   // Limited range
		Size:            0.7,
	}
}

func NewTurretCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1.5,
		BulletSpeedMod:  1.0,
		BulletDamageMod: 1.0,
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            1.0,
	}
}

func NewMachineGunCannon() CannonStats {
	return CannonStats{
		ReloadTime:      0.3,
		BulletSpeedMod:  0.7,
		BulletDamageMod: 0.3,
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            0.7,
	}
}

func NewChaseCannon() CannonStats {
	return CannonStats{
		ReloadTime:      1,
		BulletSpeedMod:  1.2,
		BulletDamageMod: 0.7,
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            0.7,
	}
}

func NewBigCannon() CannonStats {
	return CannonStats{
		ReloadTime:      2,
		BulletSpeedMod:  1,
		BulletDamageMod: 5,
		BulletCount:     1,
		SpreadAngle:     0,
		Range:           0,
		Size:            1.5,
	}
}

func NewRowingOar() CannonStats {
	return CannonStats{
		ReloadTime:      0, // No firing
		BulletSpeedMod:  0, // No bullets
		BulletDamageMod: 0, // No damage
		BulletCount:     0, // No bullets
		SpreadAngle:     0, // No spread
		Range:           0, // No range
	}
}
