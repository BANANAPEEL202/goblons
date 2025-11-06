# Input System Architecture

## Overview
This document describes the reworked input handling system that separates movement inputs from action inputs, implements proper deduplication, and adds server-side cooldown management.

## Problem Statement
The previous input system had several issues:
1. **Input Loss**: Actions weren't reliably processed by the backend
2. **Duplicate Processing**: Key holds could trigger multiple actions
3. **Inconsistent Cooldowns**: Client-side cooldowns weren't synchronized with server
4. **Complex State Management**: Multiple cooldown tracking mechanisms across frontend and backend

## Solution Architecture

### Frontend (client.js)

#### Input Structure
```javascript
{
  type: 'input',
  // Continuous movement state
  up: bool,
  down: bool,
  left: bool,
  right: bool,
  // Event-based actions (processed once)
  actions: [
    { type: 'statUpgrade', sequence: 1, data: 'hullStrength' },
    { type: 'toggleAutofire', sequence: 2, data: '' }
  ],
  mouse: { x: float, y: float }
}
```

#### Action System
- **Sequence Numbers**: Each action gets a unique, incrementing sequence number
- **Deduplication**: Server tracks last processed sequence to avoid duplicates
- **Client-side Cooldowns**: Prevent spamming before server response
- **Action Queue**: Actions are queued and cleared after sending

#### queueAction(actionType, data)
```javascript
// Checks cooldown
// Assigns sequence number
// Adds to actions array
// Tracks pending action timestamp
```

### Backend (world.go, types.go)

#### Player State
```go
type Player struct {
    // ...
    LastProcessedAction uint32            // Deduplication
    ActionCooldowns     map[string]time.Time // Server-side cooldowns
}
```

#### processPlayerActions()
For each action in the input:
1. **Check Sequence**: Skip if `action.Sequence <= player.LastProcessedAction`
2. **Check Cooldown**: Enforce server-side cooldown per action type
3. **Process Action**: Execute the action (upgrade, toggle, etc.)
4. **Update State**: Record sequence and cooldown timestamp

### Action Types

#### statUpgrade
- **Cooldown**: 200ms
- **Data**: stat type (e.g., "hullStrength", "moveSpeed")
- **Processing**: Calls `player.BuyUpgrade(statType)`

#### toggleAutofire
- **Cooldown**: 500ms
- **Data**: empty string
- **Processing**: Toggles `player.AutofireEnabled`

## Key Benefits

1. **No Input Loss**: Actions are queued and processed reliably
2. **Single Processing**: Sequence numbers prevent duplicate execution
3. **Server Authority**: Server enforces cooldowns, preventing client manipulation
4. **Simpler Frontend**: No complex cooldown management in client
5. **Extensible**: Easy to add new action types

## Movement vs Actions

### Movement (Continuous State)
- **Keys**: WASD, Arrow keys
- **Behavior**: Hold = continuous effect
- **Processing**: Every frame on both client and server
- **Use Case**: Ship movement, turning

### Actions (Single-Fire Events)
- **Keys**: 1-8 (upgrades), R (autofire)
- **Behavior**: Press = single effect
- **Processing**: Once per keypress, with cooldown
- **Use Case**: Upgrades, toggles, purchases

## Migration Notes

### Legacy Support
The system maintains backward compatibility with old input fields:
- `input.toggleAutofire` (bool)
- `input.statUpgradeType` (string)

These will be processed if `input.actions` is empty, allowing gradual migration.

### Client-side Prediction
For actions like autofire toggle, the client can optimistically update the UI:
```javascript
this.clientAutofireState = !this.gameState.myPlayer.autofireEnabled;
// Clear after 1s when server response arrives
```

## Testing Checklist

- [ ] Stat upgrades (keys 1-8) work with rapid presses
- [ ] Stat upgrades respect 200ms cooldown
- [ ] Autofire toggle (R key) works reliably
- [ ] Autofire toggle respects 500ms cooldown
- [ ] Holding upgrade keys doesn't spam upgrades
- [ ] Movement (WASD) is smooth and continuous
- [ ] No duplicate purchases when spamming keys
- [ ] Server logs show proper action sequence tracking
- [ ] Cooldown messages appear when appropriate

## Future Enhancements

1. **Action Acknowledgment**: Server could send ACK to clear pending actions
2. **Action Queue Limit**: Prevent client from queuing too many actions
3. **Per-Player Rate Limiting**: Detect and throttle spam attempts
4. **Action History**: Track action history for debugging/analytics
