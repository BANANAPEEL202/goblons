package game

import (
	"github.com/vmihailenco/msgpack/v5"
	"log"
)

// sendAvailableUpgrades sends available upgrades to a specific client
func (client *Client) sendAvailableUpgrades() {
	upgrades := make(map[string][]UpgradeInfo)

	// Get available upgrades for each type and convert to simplified format
	upgradeTypes := []moduleType{UpgradeTypeSide, UpgradeTypeTop, UpgradeTypeFront, UpgradeTypeRear}

	for _, upgradeType := range upgradeTypes {
		availableUpgrades := client.Player.ShipConfig.GetAvailableModules(upgradeType)
		upgradeInfos := make([]UpgradeInfo, 0, len(availableUpgrades))

		for _, upgrade := range availableUpgrades {
			if upgrade != nil {
				upgradeInfos = append(upgradeInfos, UpgradeInfo{
					Name: upgrade.Name,
					Type: string(upgrade.Type),
				})
			}
		}

		upgrades[string(upgradeType)] = upgradeInfos
	}

	upgradesMsg := AvailableUpgradesMsg{
		Type:     "availableUpgrades",
		Upgrades: upgrades,
	}

	data, err := msgpack.Marshal(upgradesMsg)
	if err != nil {
		log.Printf("Error marshaling available upgrades message: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		// Channel full, skip
		log.Printf("Could not send available upgrades to client %d", client.ID)
	}
}

func (client *Client) sendGameEvent(event GameEventMsg) {
	event.Type = MsgTypeGameEvent

	data, err := msgpack.Marshal(event)
	if err != nil {
		log.Printf("Error marshaling game event message: %v", err)
		return
	}

	select {
	case client.Send <- data:
	default:
		log.Printf("Could not send game event to client %d", client.ID)
	}
}

func (client *Client) sendWelcomeMessage() {
	welcomeMsg := WelcomeMsg{
		Type:     MsgTypeWelcome,
		PlayerId: client.ID,
	}

	data, err := msgpack.Marshal(welcomeMsg)
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
