package game

import (
	"fmt"
	"math"
	"time"
)

const (
	botCount                     = 3
	botGuardRadius       float32 = 100.0
	botAggroRadius       float32 = 1000.0
	botTargetDistance    float32 = 500.0
	botPreferredDistance float32 = 200.0
	botDistanceSlack     float32 = 80.0
	botMaxHealth         int     = 160
	botSideCannonsCount  int     = 2
	botTopTurretCount    int     = 1
	botDecisionInterval          = 250 * time.Millisecond
	botCannonDamageLevel         = 3
	botCannonRangeLevel          = 2
	botReloadSpeedLevel          = 2
	botMoveSpeedLevel            = 1
	botTurnSpeedLevel            = 1
)

const (
	botAreaMinX float32 = 0
	botAreaMaxX float32 = WorldWidth
	botAreaMinY float32 = 0
	botAreaMaxY float32 = WorldHeight
)

var botSpawnPoints = []Position{
	{X: float32(WorldWidth * 0.5), Y: float32(WorldHeight * 0.5)},
	{X: float32(WorldWidth*0.5) + 220, Y: float32(WorldHeight*0.5) - 220},
	{X: float32(WorldWidth*0.5) - 220, Y: float32(WorldHeight*0.5) + 220},
}

var botColors = []string{"#5B73FF", "#FF6F61", "#48C9B0"}

func (w *World) spawnInitialBots() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.botsSpawned {
		return
	}

	now := time.Now()

	spawnCount := botCount
	if spawnCount > len(botSpawnPoints) {
		spawnCount = len(botSpawnPoints)
	}

	for i := 0; i < spawnCount; i++ {
		id := w.nextID
		w.nextID++

		player := NewPlayer(id)
		player.IsBot = true
		player.Name = fmt.Sprintf("Guardian %d", i+1)
		player.Color = botColors[i%len(botColors)]
		player.Score = 0
		player.Coins = 0
		player.Level = 1
		player.AvailableUpgrades = 0
		player.X = botSpawnPoints[i].X
		player.Y = botSpawnPoints[i].Y
		player.Angle = 0
		player.Health = botMaxHealth
		player.MaxHealth = botMaxHealth
		player.AutofireEnabled = true
		player.LastRegenTime = now
		player.LastCollisionDamage = now

		w.applyBotLoadout(player)

		orbitDir := 1
		if i%2 == 1 {
			orbitDir = -1
		}

		bot := &Bot{
			ID:                id,
			Player:            player,
			GuardCenter:       botSpawnPoints[i],
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

	w.botsSpawned = true
}

func (w *World) applyBotLoadout(player *Player) {
	baseLength := float32(PlayerSize*1.2) * 0.5
	baseWidth := float32(PlayerSize * 0.8)

	InitializeStatUpgrades(player)
	ForceStatUpgrades(player, map[StatUpgradeType]int{
		StatUpgradeCannonDamage: botCannonDamageLevel,
		StatUpgradeCannonRange:  botCannonRangeLevel,
		StatUpgradeReloadSpeed:  botReloadSpeedLevel,
		StatUpgradeMoveSpeed:    botMoveSpeedLevel,
		StatUpgradeTurnSpeed:    botTurnSpeedLevel,
	})

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

func (w *World) updateBots() {
	if len(w.bots) == 0 {
		return
	}

	now := time.Now()
	for _, bot := range w.bots {
		w.updateBot(bot, now)
	}
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

	var desiredAngle float32
	hasDesiredAngle := false
	target := w.players[bot.TargetPlayerID]
	if bot.TargetPlayerID != 0 && target != nil {
		player.AutofireEnabled = true
		bot.Input.Mouse.X = target.X
		bot.Input.Mouse.Y = target.Y

		angleToTarget := float32(math.Atan2(float64(target.Y-player.Y), float64(target.X-player.X)))
		distance := float32(math.Hypot(float64(target.X-player.X), float64(target.Y-player.Y)))

		if distance > bot.PreferredDistance+botDistanceSlack {
			desiredAngle = angleToTarget
		} else if distance < bot.PreferredDistance-botDistanceSlack {
			desiredAngle = angleToTarget + float32(bot.OrbitDirection)*float32(math.Pi*0.75)
		} else {
			desiredAngle = angleToTarget + float32(bot.OrbitDirection)*float32(math.Pi/2)
		}
		hasDesiredAngle = true

		if !bot.inAllowedZone(target.X, target.Y) {
			bot.TargetPlayerID = 0
			bot.NextDecision = now.Add(botDecisionInterval)
		}
	} else {
		dx := bot.GuardCenter.X - player.X
		dy := bot.GuardCenter.Y - player.Y
		distance := float32(math.Hypot(float64(dx), float64(dy)))
		angleToCenter := float32(math.Atan2(float64(dy), float64(dx)))

		bot.Input.Mouse.X = bot.GuardCenter.X
		bot.Input.Mouse.Y = bot.GuardCenter.Y

		if distance > bot.GuardRadius*0.5 {
			desiredAngle = angleToCenter
		} else if distance > bot.GuardRadius*0.25 {
			desiredAngle = angleToCenter + float32(bot.OrbitDirection)*float32(math.Pi/3)
		} else {
			desiredAngle = angleToCenter + float32(bot.OrbitDirection)*float32(math.Pi/2)
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

	turnResponseRange := float32(math.Pi / 2)
	if turnResponseRange <= 0 {
		turnResponseRange = 1
	}
	desiredTurn := clampFloat32(angleDiff/turnResponseRange, -1, 1)
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
	bestDistance := float32(math.MaxFloat32)

	for id, candidate := range w.players {
		if candidate == nil || candidate.IsBot || candidate.State != StateAlive {
			continue
		}
		if !bot.inAllowedZone(candidate.X, candidate.Y) {
			continue
		}

		distance := float32(math.Hypot(float64(candidate.X-bot.Player.X), float64(candidate.Y-bot.Player.Y)))
		if distance < bestDistance && distance <= bot.TargetDistance {
			bestDistance = distance
			bestID = id
		}
	}

	return bestID
}

func (bot *Bot) inAllowedZone(x, y float32) bool {
	if x < botAreaMinX || x > botAreaMaxX || y < botAreaMinY || y > botAreaMaxY {
		return false
	}

	dx := x - bot.GuardCenter.X
	dy := y - bot.GuardCenter.Y
	return float32(math.Hypot(float64(dx), float64(dy))) <= bot.AggroRadius
}

func (w *World) respawnBot(bot *Bot, now time.Time) {
	player := bot.Player
	if player == nil {
		return
	}

	w.applyBotLoadout(player)

	player.Health = botMaxHealth
	player.MaxHealth = botMaxHealth
	player.State = StateAlive
	player.X = bot.GuardCenter.X
	player.Y = bot.GuardCenter.Y
	player.VelX = 0
	player.VelY = 0
	player.Angle = 0
	player.AngularVelocity = 0
	player.AutofireEnabled = true
	player.RespawnTime = time.Time{}
	player.LastRegenTime = now
	player.LastCollisionDamage = now

	bot.TargetPlayerID = 0
	bot.NextDecision = now.Add(botDecisionInterval)
}

func normalizeAngle(angle float32) float32 {
	for angle > float32(math.Pi) {
		angle -= float32(2 * math.Pi)
	}
	for angle < -float32(math.Pi) {
		angle += float32(2 * math.Pi)
	}
	return angle
}

func clampFloat32(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
