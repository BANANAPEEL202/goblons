package game

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

const (
	botCount                     = 5
	botGuardRadius       float64 = 500.0
	botAggroRadius       float64 = 1500.0
	botTargetDistance    float64 = 700.0
	botPreferredDistance float64 = 200.0
	botDistanceSlack     float64 = 80.0
	botSideCannonsCount  int     = 2
	botTopTurretCount    int     = 1
	botDecisionInterval          = 250 * time.Millisecond
	botCannonDamageLevel         = 5
	botCannonRangeLevel          = 5
	botReloadSpeedLevel          = 5
	botMoveSpeedLevel            = 0
	botTurnSpeedLevel            = 0
	botHealthLevel               = 5
	botRegenLevel                = 5
)

const (
	botAreaMinX float64 = 0
	botAreaMaxX float64 = WorldWidth
	botAreaMinY float64 = 0
	botAreaMaxY float64 = WorldHeight
)

var botColors = []string{"#5B73FF", "#FF6F61", "#48C9B0"}

func (w *World) spawnInitialBots() {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := time.Now()

	for i := 0; i < botCount; i++ {
		id := w.nextPlayerID
		w.nextPlayerID++

		player := NewPlayer(id)
		player.IsBot = true
		player.Name = fmt.Sprintf("Guardian %d", i+1)
		player.Color = botColors[i%len(botColors)]
		player.Score = 2000
		player.Coins = 2000
		player.Experience = 2000
		player.Level = 25
		player.AvailableUpgrades = 0

		// Random spawn position with some padding from edges
		spawnPos := Position{
			X: float64(rand.Intn(int(WorldWidth-200)) + 100),
			Y: float64(rand.Intn(int(WorldHeight-200)) + 100),
		}

		player.X = spawnPos.X
		player.Y = spawnPos.Y
		player.Angle = 0
		player.AutofireEnabled = true
		player.LastCollisionDamage = now

		w.applyBotLoadout(player)

		orbitDir := 1
		if i%2 == 1 {
			orbitDir = -1
		}

		bot := &Bot{
			ID:                id,
			Player:            player,
			GuardCenter:       spawnPos,
			GuardRadius:       botGuardRadius,
			TargetDistance:    botTargetDistance,
			AggroRadius:       botAggroRadius,
			PreferredDistance: botPreferredDistance,
			OrbitDirection:    orbitDir,
			DesiredAngle:      0,
		}

		w.players[id] = player
		w.bots[id] = bot
	}
}

func (w *World) applyBotLoadout(player *Player) {
	baseLength := float64(PlayerSize*1.2) * 0.5
	baseWidth := float64(PlayerSize * 0.8)

	player.InitializeStatUpgrades()
	ForceStatUpgrades(player, map[UpgradeType]int{
		StatUpgradeCannonDamage: botCannonDamageLevel,
		StatUpgradeCannonRange:  botCannonRangeLevel,
		StatUpgradeReloadSpeed:  botReloadSpeedLevel,
		StatUpgradeMoveSpeed:    botMoveSpeedLevel,
		StatUpgradeTurnSpeed:    botTurnSpeedLevel,
		StatUpgradeHullStrength: botHealthLevel,
		StatUpgradeAutoRepairs:  botRegenLevel,
	})
	player.Modifiers.MoveSpeedMultiplier = 0.8 // Slightly slower base speed for bots
	player.Health = player.MaxHealth

	config := ShipConfiguration{
		SideUpgrade:  NewBasicSideCannons(botSideCannonsCount),
		TopUpgrade:   NewBasicTurrets(botTopTurretCount),
		FrontUpgrade: nil,
		RearUpgrade:  nil,
		ShipLength:   baseLength,
		ShipWidth:    baseWidth,
		Size:         PlayerSize,
	}

	config.CalculateShipDimensions()
	config.UpdateUpgradePositions()

	player.ShipConfig = config
}

func ForceStatUpgrades(player *Player, upgrades map[UpgradeType]int) {
	for upgradeType, level := range upgrades {
		player.Upgrades[upgradeType] = Upgrade{
			Type:  upgradeType,
			Level: level,
		}
	}
	player.updateModifiers()
}

func (w *World) updateBots() {
	if len(w.bots) == 0 {
		return
	}

	now := time.Now()
	for _, bot := range w.bots {
		w.updateBot(bot, now)
	}

	w.handleBotRespawns()
}

func (w *World) updateBot(bot *Bot, now time.Time) {
	player := bot.Player
	if player == nil || player.State != StateAlive {
		return
	}

	bot.Input = InputMsg{}
	bot.Input.Up = true
	player.AutofireEnabled = false

	if bot.OrbitDirection == 0 {
		bot.OrbitDirection = 1
	}

	// Drop invalid targets when they leave the engagement rules.
	if bot.TargetPlayerID != 0 {
		target := w.players[bot.TargetPlayerID]
		if target == nil || target.IsBot || target.State != StateAlive || !bot.inAllowedZone(target.X, target.Y) {
			bot.TargetPlayerID = 0
		}
	}

	if (bot.TargetPlayerID == 0 && (bot.NextDecision.IsZero() || now.After(bot.NextDecision))) || (bot.TargetPlayerID != 0 && now.After(bot.NextDecision)) {
		previous := bot.TargetPlayerID
		bot.TargetPlayerID = w.findBotTarget(bot)
		if bot.TargetPlayerID != 0 && bot.TargetPlayerID != previous {
			bot.DesiredAngle = player.Angle
		}
		bot.NextDecision = now.Add(botDecisionInterval)
	}

	var desiredAngle float64
	hasDesiredAngle := false
	target := w.players[bot.TargetPlayerID]
	if bot.TargetPlayerID != 0 && target != nil {
		player.AutofireEnabled = true
		bot.Input.Mouse.X = target.X
		bot.Input.Mouse.Y = target.Y

		angleToTarget := float64(math.Atan2(float64(target.Y-player.Y), float64(target.X-player.X)))
		distance := float64(math.Hypot(float64(target.X-player.X), float64(target.Y-player.Y)))

		if distance > bot.PreferredDistance+botDistanceSlack {
			desiredAngle = angleToTarget
		} else if distance < bot.PreferredDistance-botDistanceSlack {
			desiredAngle = angleToTarget + float64(bot.OrbitDirection)*float64(math.Pi*0.75)
		} else {
			desiredAngle = angleToTarget + float64(bot.OrbitDirection)*float64(math.Pi/2)
		}
		hasDesiredAngle = true

		if !bot.inAllowedZone(target.X, target.Y) {
			bot.TargetPlayerID = 0
			bot.NextDecision = now.Add(botDecisionInterval)
		}
	} else {
		dx := bot.GuardCenter.X - player.X
		dy := bot.GuardCenter.Y - player.Y
		distance := float64(math.Hypot(float64(dx), float64(dy)))
		angleToCenter := float64(math.Atan2(float64(dy), float64(dx)))

		bot.Input.Mouse.X = bot.GuardCenter.X
		bot.Input.Mouse.Y = bot.GuardCenter.Y

		if distance > bot.GuardRadius*0.5 {
			desiredAngle = angleToCenter
		} else if distance > bot.GuardRadius*0.25 {
			desiredAngle = angleToCenter + float64(bot.OrbitDirection)*float64(math.Pi/3)
		} else {
			desiredAngle = angleToCenter + float64(bot.OrbitDirection)*float64(math.Pi/2)
		}
		hasDesiredAngle = true
	}

	if !hasDesiredAngle {
		desiredAngle = player.Angle
	}

	desiredAngle = normalizeAngle(desiredAngle)
	bot.DesiredAngle = desiredAngle

	angleDiff := normalizeAngle(desiredAngle - player.Angle)
	if math.Abs(float64(angleDiff)) < 0.04 {
		angleDiff = 0
	}

	turnResponseRange := float64(math.Pi / 2)
	if turnResponseRange <= 0 {
		turnResponseRange = 1
	}
	desiredTurn := clampfloat64(angleDiff/turnResponseRange, -1, 1)
	const steeringSmoothing = 0.18
	bot.TurnIntent += (desiredTurn - bot.TurnIntent) * steeringSmoothing

	const steeringDeadzone = 0.1
	if bot.TurnIntent > steeringDeadzone {
		bot.Input.Right = true
	} else if bot.TurnIntent < -steeringDeadzone {
		bot.Input.Left = true
	}

	w.updatePlayer(player, &bot.Input)
}

func (w *World) findBotTarget(bot *Bot) uint32 {
	var bestID uint32
	bestDistance := float64(math.MaxFloat64)

	for id, candidate := range w.players {
		if candidate == nil || candidate.IsBot || candidate.State != StateAlive {
			continue
		}
		if !bot.inAllowedZone(candidate.X, candidate.Y) {
			continue
		}

		distance := float64(math.Hypot(float64(candidate.X-bot.Player.X), float64(candidate.Y-bot.Player.Y)))
		if distance < bestDistance && distance <= bot.TargetDistance {
			bestDistance = distance
			bestID = id
		}
	}

	return bestID
}

func (bot *Bot) inAllowedZone(x, y float64) bool {
	if x < botAreaMinX || x > botAreaMaxX || y < botAreaMinY || y > botAreaMaxY {
		return false
	}

	dx := x - bot.GuardCenter.X
	dy := y - bot.GuardCenter.Y
	return float64(math.Hypot(float64(dx), float64(dy))) <= bot.AggroRadius
}

func (w *World) respawnBot(bot *Bot, now time.Time) {
	player := bot.Player
	if player == nil {
		return
	}

	w.applyBotLoadout(player)

	// Random respawn position with some padding from edges
	spawnPos := Position{
		X: float64(rand.Intn(int(WorldWidth-200)) + 100),
		Y: float64(rand.Intn(int(WorldHeight-200)) + 100),
	}

	player.State = StateAlive
	player.X = spawnPos.X
	player.Y = spawnPos.Y
	player.VelX = 0
	player.VelY = 0
	player.Angle = 0
	player.AutofireEnabled = true
	player.RespawnTime = time.Time{}
	player.LastCollisionDamage = now

	// Update guard center to new spawn location
	bot.GuardCenter = spawnPos
	bot.TargetPlayerID = 0
	bot.NextDecision = now.Add(botDecisionInterval)
}

func normalizeAngle(angle float64) float64 {
	for angle > float64(math.Pi) {
		angle -= float64(2 * math.Pi)
	}
	for angle < -float64(math.Pi) {
		angle += float64(2 * math.Pi)
	}
	return angle
}

func clampfloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
