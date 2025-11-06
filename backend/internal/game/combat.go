package game

import (
	"log"
	"time"
)

// KillCause represents the origin of lethal damage for logging and reward logic.
type KillCause string

const (
	KillCauseBullet    KillCause = "bullet"
	KillCauseCollision KillCause = "collision"
	KillCauseRam       KillCause = "ram"
)

// ApplyDamage subtracts health from the target and handles death side-effects.
func (gm *GameMechanics) ApplyDamage(target *Player, damage int, attacker *Player, cause KillCause, now time.Time) bool {
	if target == nil || target.State != StateAlive || damage <= 0 {
		return false
	}

	if damage == 0 {
		log.Printf("Warning: Attempted to apply zero damage to Player %d", target.ID)
		damage = 1 // Ensure at least 1 damage is applied
	}

	target.Health -= damage
	if target.Health > 0 {
		return false
	}

	gm.handlePlayerDeath(target, attacker, cause, now)
	return true
}

func (gm *GameMechanics) handlePlayerDeath(victim *Player, killer *Player, cause KillCause, now time.Time) {
	victim.Health = 0
	victim.State = StateDead
	victim.RespawnTime = now.Add(time.Duration(RespawnDelay) * time.Second)

	// Track death information
	victim.DeathTime = now
	victim.ScoreAtDeath = victim.Score
	if !victim.SpawnTime.IsZero() {
		victim.SurvivalTime = now.Sub(victim.SpawnTime).Seconds()
	}

	if killer != nil {
		xpReward, coinReward := gm.calculateKillOutcome(victim)

		// Track who killed the victim
		victim.KilledBy = killer.ID
		victim.KilledByName = killer.Name

		// Apply rewards to killer
		killer.AddExperience(xpReward)
		killer.Score += xpReward
		killer.Coins += coinReward

		log.Printf("Player %d (%s) was killed by %s from Player %d (%s)",
			victim.ID, victim.Name, cause.describe(), killer.ID, killer.Name)
		log.Printf("Player %d gained %d XP and %d coins for killing Player %d (victim now has %d XP and %d coins)",
			killer.ID, xpReward, coinReward, victim.ID, victim.Experience, victim.Coins)

		if killer.ID != victim.ID && !killer.IsBot {
			if client, exists := gm.world.GetClient(killer.ID); exists {
				sendGameEvent(client, GameEventMsg{
					EventType:  "playerSunk",
					KillerID:   killer.ID,
					KillerName: killer.Name,
					VictimID:   victim.ID,
					VictimName: victim.Name,
				})
			}
		}
	} else {
		// No killer (e.g., suicide or environment)
		victim.KilledBy = 0
		victim.KilledByName = ""
		log.Printf("Player %d (%s) died due to %s", victim.ID, victim.Name, cause.describe())
	}
}

func (gm *GameMechanics) calculateKillOutcome(victim *Player) (xpReward int, coinReward int) {
	xpReward = max(victim.Experience/2, 100)
	coinReward = max(victim.Coins/2, 200)
	if coinReward > 2000 {
		coinReward = 2000
	}

	return
}

func (cause KillCause) describe() string {
	switch cause {
	case KillCauseBullet:
		return "a bullet"
	case KillCauseCollision:
		return "collision damage"
	case KillCauseRam:
		return "a ram"
	default:
		return string(cause)
	}
}
