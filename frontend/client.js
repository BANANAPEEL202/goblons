// Game constants (should match backend)
const WorldWidth = 5000.0;
const WorldHeight = 5000.0;
const PRESET_COLORS = ['#FF0040', '#00FF80', '#0080FF', '#FF8000', '#8000FF'];
const NAME_POOL = ['Pirate', 'Buccaneer', 'Sailor', 'Captain', 'Admiral', 'Navigator', 'Corsair', 'Raider'];

class GameClient {
  constructor(options = {}) {
    this.playerConfig = {
      name: sanitizePlayerName(options.playerName),
      color: sanitizeHexColor(options.playerColor),
    };
    this.autoConnect = options.autoConnect !== false;
    this.shouldStartGame = options.shouldStartGame || false; // Flag to auto-start game
    this.hasStartedGame = false; // Track if player has already started the game
    this.canvas = document.getElementById('game');
    this.ctx = this.canvas.getContext('2d');
    this.socket = null;
    this.myPlayerId = null;
    this.gameState = {
      players: [],
      items: [],
      bullets: [],
      myPlayer: null
    };
    this.input = {
      type: 'input',
      // Movement inputs (continuous state)
      up: false,
      down: false,
      left: false, 
      right: false,
      // Action queue (event-based with sequence numbers)
      actions: [],
      // Mouse position
      mouse: { x: 0, y: 0 },
      // Legacy fields (kept for compatibility during transition)
      shootLeft: false,
      shootRight: false,
      upgradeCannons: false,
      downgradeCannons: false,
      upgradeScatter: false,
      downgradeScatter: false,
      upgradeTurrets: false,
      downgradeTurrets: false,
      debugLevelUp: false,
      selectUpgrade: '',
      upgradeChoice: '',
      statUpgradeType: '',
      toggleAutofire: false,
      manualFire: false,
      requestRespawn: false,
    };
    
    // Action sequencing for deduplication
    this.actionSequence = 0;
    this.pendingActions = new Map(); // actionType -> {sequence, timestamp}
    this.actionCooldowns = {
      statUpgrade: 100,     // 150ms between stat upgrades (matches backend)
      toggleAutofire: 400,  // 400ms between autofire toggles (matches backend)
    };
    
    // Ship physics properties for client-side prediction

    this.shipPhysics = {
      angle: 0,           // Current facing direction (radians)
      velocity: { x: 0, y: 0 },  // Current velocity
      acceleration: 20000000,   // Forward acceleration (doubled for 30 TPS to match server)
      deceleration: 0.84,  // Drag/friction factor (adjusted for 30 TPS to match server)
      turnSpeed: 0.08,      // How fast the ship turns (doubled for 30 TPS to match server)
      maxSpeed: 4.0        // Maximum speed (already matches server at 30 TPS)
    };
    
    this.camera = { x: 0, y: 0, targetX: 0, targetY: 0 };
    this.isConnected = false;
    this.screenWidth = window.innerWidth;
    this.screenHeight = window.innerHeight;
    
    // Input sending interval
    this.inputSendInterval = null;
    this.isSendingInput = false; // Prevent concurrent sends
    this.lastInputSendTime = 0; // Track last send time for throttling
    
    // UI state for upgrade system
    this.upgradeUI = {
      selectedUpgradeType: null, // 'side', 'top', 'front', 'rear'
      availableUpgrades: {},     // stores available upgrades for each type
      pendingUpgrade: false,     // prevents multiple upgrade selections
      optionPositions: {},       // stores click positions for upgrade options
      upgradeSent: false,        // tracks if current upgrade was successfully sent
    };
    
    // Track last mouse screen position for camera movement updates
    this.lastMouseScreen = { x: 0, y: 0 };
    this.lastCameraPos = { x: 0, y: 0 };
    
    // Client-side prediction
    this.predictedPlayerPos = { x: 0, y: 0 };
    this.lastPredictionUpdate = Date.now();

    this.controlsLocked = true;
    this.pendingConnectConfig = null;
    
    // Death screen state
    this.deathScreen = {
      visible: false,
      score: 0,
      survivalTime: 0,
      killerName: '',
      respawnButtonBounds: null
    };

    this.killNotifications = [];
    
    this.resizeCanvas();
    this.init();
  }

  // Helper function for rounded rectangles
  drawRoundedRect(x, y, width, height, radius) {
    this.ctx.beginPath();
    this.ctx.moveTo(x + radius, y);
    this.ctx.lineTo(x + width - radius, y);
    this.ctx.quadraticCurveTo(x + width, y, x + width, y + radius);
    this.ctx.lineTo(x + width, y + height - radius);
    this.ctx.quadraticCurveTo(x + width, y + height, x + width - radius, y + height);
    this.ctx.lineTo(x + radius, y + height);
    this.ctx.quadraticCurveTo(x, y + height, x, y + height - radius);
    this.ctx.lineTo(x, y + radius);
    this.ctx.quadraticCurveTo(x, y, x + radius, y);
    this.ctx.closePath();
  }

  init() {
    this.setupEventListeners();
    if (this.autoConnect) {
      this.connect();
    }
    this.startGameLoop();
  }

  resizeCanvas() {
    const dpr = window.devicePixelRatio || 1;
    const displayWidth = window.innerWidth;
    const displayHeight = window.innerHeight;
    
    // Store the current transform matrix
    this.ctx.save();
    
    // Reset the current transform
    this.ctx.setTransform(1, 0, 0, 1, 0, 0);
    
    this.canvas.width = displayWidth * dpr;
    this.canvas.height = displayHeight * dpr;
    
    this.canvas.style.width = displayWidth + 'px';
    this.canvas.style.height = displayHeight + 'px';
    
    // Scale the context for high DPI displays
    this.ctx.scale(dpr, dpr);
    
    // Store screen dimensions for rendering calculations
    this.screenWidth = displayWidth;
    this.screenHeight = displayHeight;
  }

  setupEventListeners() {
    // Keyboard input
    document.addEventListener('keydown', (e) => {
      this.handleKeyDown(e);
    }, { passive: false });
    
    document.addEventListener('keyup', (e) => {
      this.handleKeyUp(e);
    }, { passive: false });
    
    // Mouse input
    this.canvas.addEventListener('mousemove', (e) => {
      if (this.controlsLocked) {
        return;
      }

      const rect = this.canvas.getBoundingClientRect();
      const screenX = e.clientX - rect.left;
      const screenY = e.clientY - rect.top;
      
      // Store screen coordinates for camera movement updates
      this.lastMouseScreen.x = screenX;
      this.lastMouseScreen.y = screenY;
      
      // Convert screen coordinates to world coordinates
      // Account for camera position: camera.x/y represents the top-left corner of the view
      this.updateMouseWorldCoords();
      
      // Send input whenever mouse moves (for turret aiming)
      this.sendInput();
    });
    
    // Mouse click handling for upgrade UI and manual firing
    this.canvas.addEventListener('click', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const screenX = e.clientX - rect.left;
      const screenY = e.clientY - rect.top;
      
      // Check death screen respawn button first
      if (this.deathScreen.visible && this.deathScreen.respawnButtonBounds) {
        const bounds = this.deathScreen.respawnButtonBounds;
        if (screenX >= bounds.x && screenX <= bounds.x + bounds.width &&
            screenY >= bounds.y && screenY <= bounds.y + bounds.height) {
          this.handleRespawn();
          return;
        }
      }
      
      if (this.controlsLocked) {
        console.log('Controls are locked; ignoring click.');
        return;
      }
      
      // Try to handle upgrade UI click first
      const handledByUI = this.handleUpgradeUIClick(screenX, screenY);
      
      // If not handled by UI, trigger manual fire
      if (!handledByUI) {
        this.input.manualFire = true;
        this.sendInput();
        // Clear the flag after a short delay
        setTimeout(() => {
          this.input.manualFire = false;
        }, 50);
      }
    });

    // Handle window resize
    window.addEventListener('resize', () => {
      this.resizeCanvas();
    });

    // Handle fullscreen toggle
    document.addEventListener('keydown', (e) => {
      if (e.key === 'F11') {
        e.preventDefault();
        this.toggleFullscreen();
      }
    });
  }

  toggleFullscreen() {
    if (!document.fullscreenElement) {
      document.documentElement.requestFullscreen().catch(console.error);
    } else {
      document.exitFullscreen().catch(console.error);
    }
  }

  connect(newConfig = {}, options = {}) {
    if (newConfig.playerName !== undefined || newConfig.playerColor !== undefined) {
      const updatedName =
        newConfig.playerName !== undefined ? sanitizePlayerName(newConfig.playerName) : this.playerConfig.name;
      const updatedColor =
        newConfig.playerColor !== undefined ? sanitizeHexColor(newConfig.playerColor) : this.playerConfig.color;
      this.playerConfig = {
        name: updatedName,
        color: updatedColor,
      };
    }

    const isActiveSocket =
      this.socket &&
      (this.socket.readyState === WebSocket.OPEN || this.socket.readyState === WebSocket.CONNECTING);

    if (isActiveSocket) {
      if (options.force === true) {
        this.pendingConnectConfig = { ...this.playerConfig };
        try {
          this.socket.close();
        } catch (err) {
          console.error('Error closing socket before reconnect:', err);
          this.socket = null;
          this.connect(this.pendingConnectConfig || {}, {});
        }
      } else {
        // Already connected; nothing more to do.
      }
      return;
    }

    this.updateConnectionStatus(false);

    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    const params = new URLSearchParams();

    if (this.playerConfig.name) {
      params.set('name', this.playerConfig.name);
    }
    if (this.playerConfig.color) {
      params.set('color', this.playerConfig.color);
    }

    let wsUrl = `${protocol}//${location.host}/ws`;
    const query = params.toString();
    if (query) {
      wsUrl += `?${query}`;
    }

    try {
      this.socket = new WebSocket(wsUrl);
    } catch (err) {
      console.error('WebSocket creation failed:', err);
      setTimeout(() => this.connect(), 3000);
      return;
    }
    
    this.socket.onopen = () => {
      console.log('Connected to server');
      this.isConnected = true;
      this.pendingConnectConfig = null;
      this.updateConnectionStatus(true);
      
      // Start regular input sending interval (30 times per second to match server tick rate)
      if (this.inputSendInterval) {
        clearInterval(this.inputSendInterval);
      }
      this.inputSendInterval = setInterval(() => {
        // Send input regularly if we have any movement state or queued actions
        if (!this.controlsLocked && (this.input.up || this.input.down || this.input.left || 
            this.input.right || this.input.actions.length > 0)) {
          this.sendInput();
        }
      }, 33); // ~30 FPS (server tick rate)
      
      // If player has started the game before or should start now, send startGame
      if (this.shouldStartGame || this.hasStartedGame) {
        this.sendStartGame();
        this.shouldStartGame = false; // Reset flag
      }
      
      if (!this.controlsLocked) {
        this.sendInput();
      }
    };
    
    this.socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.handleMessage(data);
    };
    
    this.socket.onclose = () => {
      console.log('Disconnected from server');
      this.isConnected = false;
      this.socket = null;
      this.updateConnectionStatus(false);
      
      // Clear input send interval
      if (this.inputSendInterval) {
        clearInterval(this.inputSendInterval);
        this.inputSendInterval = null;
      }
      
      const reconnectConfig = this.pendingConnectConfig;
      this.pendingConnectConfig = null;
      const delay = reconnectConfig ? 150 : 3000;
      setTimeout(() => {
        this.connect(reconnectConfig || {});
      }, delay);
    };
    
    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  handleMessage(data) {
    switch (data.type) {
      case 'welcome':
        // Server tells us our player ID
        console.log('Received welcome message, our player ID is:', data.playerId);
        this.myPlayerId = data.playerId;
        break;
        
      case 'availableUpgrades':
        // Server sends us available upgrades
        this.upgradeUI.availableUpgrades = data.upgrades || {};
        // Reset pending upgrade flag since server has processed our request
        this.upgradeUI.pendingUpgrade = false;
        // Clear the upgrade inputs since server has processed them
        this.input.selectUpgrade = '';
        this.input.upgradeChoice = '';
        this.upgradeUI.upgradeSent = false;
        break;
        
      case 'snapshot':
        this.gameState.players = data.players || [];
        this.gameState.items = data.items || [];
        this.gameState.bullets = data.bullets || [];
        
        // Find our player by the ID we received in the welcome message
        if (this.myPlayerId) {
          const serverPlayer = this.gameState.players.find(p => p.id === this.myPlayerId);
          if (serverPlayer) {
            // Initialize our player if this is the first time we found them
            if (!this.gameState.myPlayer) {
              console.log('Found our player:', serverPlayer);
              this.gameState.myPlayer = serverPlayer;
              
              // Initialize predicted position and ship physics with server data
              this.predictedPlayerPos.x = serverPlayer.x;
              this.predictedPlayerPos.y = serverPlayer.y;
              
              if (serverPlayer.angle !== undefined) {
                this.shipPhysics.angle = serverPlayer.angle;
              }
              
              // Initialize velocity from server
              this.shipPhysics.velocity.x = serverPlayer.velX || 0;
              this.shipPhysics.velocity.y = serverPlayer.velY || 0;
            } else {
              // Update our player with server data
              const prevState = this.gameState.myPlayer.state;
              this.gameState.myPlayer = serverPlayer;
              
              // Check if player just died (transition from alive to dead)
              if (serverPlayer.state === 1 && prevState === 0 && !this.deathScreen.visible) { // State 1 = Dead, State 0 = Alive
                this.showDeathScreen(serverPlayer);
              }
              // Don't hide death screen when player respawns - let the respawn button control that
              
              // Sync angle with server
              if (serverPlayer.angle !== undefined) {
                this.shipPhysics.angle = serverPlayer.angle;
              }
              
              // Reconcile predicted position with server position
              const serverPos = { x: serverPlayer.x, y: serverPlayer.y };
              const distance = Math.sqrt(
                Math.pow(this.predictedPlayerPos.x - serverPos.x, 2) + 
                Math.pow(this.predictedPlayerPos.y - serverPos.y, 2)
              );
              
              // If prediction is too far off, snap to server position
              if (distance > 25) {
                this.predictedPlayerPos.x = serverPos.x;
                this.predictedPlayerPos.y = serverPos.y;
                // Also sync velocity to prevent further drift
                this.shipPhysics.velocity.x = serverPlayer.velX || 0;
                this.shipPhysics.velocity.y = serverPlayer.velY || 0;
              } else if (distance > 5) {
                // Gradually correct prediction towards server position
                const correctionFactor = 0.15;
                this.predictedPlayerPos.x += (serverPos.x - this.predictedPlayerPos.x) * correctionFactor;
                this.predictedPlayerPos.y += (serverPos.y - this.predictedPlayerPos.y) * correctionFactor;
              }
            }
          }
        }
        break;

      case 'deltaSnapshot':
        // Apply player deltas to existing players
        if (data.players) {
          for (const deltaPlayer of data.players) {
            const existingPlayerIndex = this.gameState.players.findIndex(p => p.id === deltaPlayer.id);
            if (existingPlayerIndex >= 0) {
              // Update existing player with delta
              this.gameState.players[existingPlayerIndex] = this.mergeDeltaPlayer(this.gameState.players[existingPlayerIndex], deltaPlayer);
            } else {
              // New player - convert delta to full player
              const fullPlayer = this.deltaToFullPlayer(deltaPlayer);
              this.gameState.players.push(fullPlayer);
            }
          }
        }
        
        // Update bullets (always full list)
        this.gameState.bullets = data.bullets || [];
        
        // Apply item deltas
        if (data.itemsAdded) {
          // Add new items
          for (const item of data.itemsAdded) {
            this.gameState.items.push(item);
          }
        }
        if (data.itemsRemoved) {
          // Remove items by ID
          const removedIds = new Set(data.itemsRemoved);
          this.gameState.items = this.gameState.items.filter(item => !removedIds.has(item.id));
        }
        
        // Find our player by the ID we received in the welcome message
        if (this.myPlayerId) {
          const serverPlayer = this.gameState.players.find(p => p.id === this.myPlayerId);
          if (serverPlayer) {
            // Initialize our player if this is the first time we found them
            if (!this.gameState.myPlayer) {
              console.log('Found our player:', serverPlayer);
              this.gameState.myPlayer = serverPlayer;
              
              // Initialize predicted position and ship physics with server data
              this.predictedPlayerPos.x = serverPlayer.x;
              this.predictedPlayerPos.y = serverPlayer.y;
              
              if (serverPlayer.angle !== undefined) {
                this.shipPhysics.angle = serverPlayer.angle;
              }
              
              // Initialize velocity from server
              this.shipPhysics.velocity.x = serverPlayer.velX || 0;
              this.shipPhysics.velocity.y = serverPlayer.velY || 0;
            } else {
              // Update our player with server data
              const prevState = this.gameState.myPlayer.state;
              this.gameState.myPlayer = serverPlayer;
              
              // Check if player just died (transition from alive to dead)
              if (serverPlayer.state === 1 && prevState === 0 && !this.deathScreen.visible) { // State 1 = Dead, State 0 = Alive
                this.showDeathScreen(serverPlayer);
              }
              // Don't hide death screen when player respawns - let the respawn button control that
              
              // Sync angle with server
              if (serverPlayer.angle !== undefined) {
                this.shipPhysics.angle = serverPlayer.angle;
              }
              
              // Reconcile predicted position with server position
              const serverPos = { x: serverPlayer.x, y: serverPlayer.y };
              const distance = Math.sqrt(
                Math.pow(this.predictedPlayerPos.x - serverPos.x, 2) + 
                Math.pow(this.predictedPlayerPos.y - serverPos.y, 2)
              );
              
              // If prediction is too far off, snap to server position
              if (distance > 25) {
                this.predictedPlayerPos.x = serverPos.x;
                this.predictedPlayerPos.y = serverPos.y;
                // Also sync velocity to prevent further drift
                this.shipPhysics.velocity.x = serverPlayer.velX || 0;
                this.shipPhysics.velocity.y = serverPlayer.velY || 0;
              } else if (distance > 5) {
                // Gradually correct prediction towards server position
                const correctionFactor = 0.15;
                this.predictedPlayerPos.x += (serverPos.x - this.predictedPlayerPos.x) * correctionFactor;
                this.predictedPlayerPos.y += (serverPos.y - this.predictedPlayerPos.y) * correctionFactor;
              }
            }
          }
        }
        break;
        
      case 'playerJoined':
        console.log(`Player ${data.playerId} joined the game`);
        break;
        
      case 'playerLeft':
        console.log(`Player ${data.playerId} left the game`);
        break;
        
      case 'gameEvent':
        this.handleGameEvent(data);
        break;
        
      default:
        console.log('Unknown message type:', data.type);
    }
  }

  handleGameEvent(data) {
    switch (data.eventType) {
      case 'playerKilled':
        if (data.victimId === this.myPlayerId) {
          console.log('You were killed!');
          // Could show death screen or respawn message
        }
        break;
      case 'playerSunk':
        if (!data.killerId || data.killerId === this.myPlayerId) {
          const victim = data.victimName && data.victimName.trim() ? data.victimName : 'Enemy';
          this.addNotification(`${victim} sunk!`);
        }
        break;
      case 'itemCollected':
        // Could add visual effects for item collection
        break;
    }
  }

  addNotification(message, duration = 3000) {
    const now = Date.now();
    this.killNotifications.push({
      message,
      expiresAt: now + duration,
      duration
    });

    if (this.killNotifications.length > 4) {
      this.killNotifications.shift();
    }
  }

  showDeathScreen(player) {
    this.deathScreen.visible = true;
    this.deathScreen.score = player.scoreAtDeath || player.score;
    this.deathScreen.survivalTime = player.survivalTime || 0;
    this.deathScreen.killerName = player.killedByName || 'Unknown';
    console.log('Death screen shown:', this.deathScreen);
  }

  handleRespawn() {
    console.log('Respawn requested');
    this.deathScreen.visible = false;
    
    // Send respawn request to server
    this.input.requestRespawn = true;
    this.sendInput();
    
    // Reset the flag after sending
    setTimeout(() => {
      this.input.requestRespawn = false;
    }, 100);
  }

  drawDeathScreen() {
    if (!this.deathScreen.visible) return;

    const ctx = this.ctx;
    const centerX = this.screenWidth / 2;
    const centerY = this.screenHeight / 2;

    // Semi-transparent overlay
    ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
    ctx.fillRect(0, 0, this.screenWidth, this.screenHeight);

    // Death screen panel
    const panelWidth = 500;
    const panelHeight = 400;
    const panelX = centerX - panelWidth / 2;
    const panelY = centerY - panelHeight / 2;

    // Panel background
    ctx.fillStyle = 'rgba(20, 20, 30, 0.7)';
    this.drawRoundedRect(panelX, panelY, panelWidth, panelHeight, 15);
    ctx.fill();

    // Panel border
    ctx.strokeStyle = '#ff4444';
    ctx.lineWidth = 3;
    this.drawRoundedRect(panelX, panelY, panelWidth, panelHeight, 15);
    ctx.stroke();

    // Title
    ctx.font = 'bold 48px Arial';
    ctx.fillStyle = '#ff4444';
    ctx.textAlign = 'center';
    ctx.fillText('YOU DIED', centerX, panelY + 80);

    // Stats
    ctx.font = 'bold 24px Arial';
    ctx.fillStyle = '#ffffff';
    
    const statsY = panelY + 150;
    const lineHeight = 45;
    
    ctx.fillText(`Score: ${this.deathScreen.score}`, centerX, statsY);
    
    const minutes = Math.floor(this.deathScreen.survivalTime / 60);
    const seconds = Math.floor(this.deathScreen.survivalTime % 60);
    const timeString = `${minutes}:${seconds.toString().padStart(2, '0')}`;
    ctx.fillText(`Survived: ${timeString}`, centerX, statsY + lineHeight);
    
    if (this.deathScreen.killerName && this.deathScreen.killerName !== '') {
      ctx.fillText(`Killed by: ${this.deathScreen.killerName}`, centerX, statsY + lineHeight * 2);
    } else {
      ctx.fillText('Killed by: Environment', centerX, statsY + lineHeight * 2);
    }

    // Respawn button
    const buttonWidth = 200;
    const buttonHeight = 50;
    const buttonX = centerX - buttonWidth / 2;
    const buttonY = panelY + panelHeight - 90;

    // Store button bounds for click detection
    this.deathScreen.respawnButtonBounds = {
      x: buttonX,
      y: buttonY,
      width: buttonWidth,
      height: buttonHeight
    };

    // Button background
    ctx.fillStyle = '#4CAF50';
    this.drawRoundedRect(buttonX, buttonY, buttonWidth, buttonHeight, 8);
    ctx.fill();

    // Button border
    ctx.strokeStyle = '#45a049';
    ctx.lineWidth = 2;
    this.drawRoundedRect(buttonX, buttonY, buttonWidth, buttonHeight, 8);
    ctx.stroke();

    // Button text
    ctx.font = 'bold 20px Arial';
    ctx.fillStyle = '#ffffff';
    ctx.fillText('RESPAWN', centerX, buttonY + 32);
  }

  drawDebugStatus() {
    if (!this.gameState.myPlayer) return;

    const player = this.gameState.myPlayer;
    const ctx = this.ctx;

    // Use values from backend-calculated debugInfo
    const debugInfo = player.debugInfo || {};
    const health = debugInfo.health || 0;
    const moveSpeedMod = debugInfo.moveSpeedModifier || 0;
    const turnSpeedMod = debugInfo.turnSpeedModifier || 0;
    const regenRate = debugInfo.regenRate || 0;
    const bodyDamage = debugInfo.bodyDamage || 0;
    const frontDps = debugInfo.frontDps || 0;
    const sideDps = debugInfo.sideDps || 0;
    const rearDps = debugInfo.rearDps || 0;
    const topDps = debugInfo.topDps || 0;
    const totalDps = debugInfo.totalDps || 0;

    // Position at bottom left, above autofire status
    const padding = 20;
    const lineHeight = 20;
    const totalLines = 10;
    const totalHeight = (totalLines - 1) * lineHeight + 16; // 16px font height
    const autofireHeight = 24; // Height of autofire status box
    const margin = 10; // Space between debug status and autofire status
    
    let y = this.screenHeight - padding - autofireHeight - margin - totalHeight;

    // Draw the debug info
    ctx.fillStyle = '#ffffff';
    ctx.font = '16px Arial';
    ctx.textAlign = 'left';
    ctx.fillText(`Health: ${health}`, padding, y); y += lineHeight;
    ctx.fillText(`Regen Rate: +${regenRate.toFixed(2)}`, padding, y); y += lineHeight;
    
    // Calculate percentage change for move speed (1.0 = 0%, 0.8 = -20%, 1.2 = +20%)
    const moveSpeedPercent = ((moveSpeedMod - 1) * 100).toFixed(0);
    const moveSpeedSign = moveSpeedPercent >= 0 ? '+' : '';
    ctx.fillText(`Move Speed Mod: ${moveSpeedSign}${moveSpeedPercent}%`, padding, y); y += lineHeight;
    
    // Calculate percentage change for turn speed
    const turnSpeedPercent = ((turnSpeedMod - 1) * 100).toFixed(0);
    const turnSpeedSign = turnSpeedPercent >= 0 ? '+' : '';
    ctx.fillText(`Turn Speed Mod: ${turnSpeedSign}${turnSpeedPercent}%`, padding, y); y += lineHeight;
    ctx.fillText(`Body Damage: +${(bodyDamage).toFixed(2)}`, padding, y); y += lineHeight;
    ctx.fillText(`Front DPS: ${frontDps.toFixed(1)}`, padding, y); y += lineHeight;
        ctx.fillText(`Top DPS: ${topDps.toFixed(1)}`, padding, y); y += lineHeight;
    ctx.fillText(`Side DPS: ${sideDps.toFixed(1)}`, padding, y); y += lineHeight;
    ctx.fillText(`Rear DPS: ${rearDps.toFixed(1)}`, padding, y); y += lineHeight;
    ctx.fillText(`Total DPS: ${totalDps.toFixed(1)}`, padding, y);
  }

  handleKeyDown(e) {
    if (this.controlsLocked) {
      return;
    }
    
    let inputChanged = false;
    
    // Handle stat upgrade keys (1-8) using new action system
    // queueAction sends immediately, so no need to set inputChanged
    if (e.key >= '1' && e.key <= '8') {
      const keyNumber = parseInt(e.key);
      const statKeyMap = {
        1: 'hullStrength',
        2: 'autoRepairs',
        3: 'cannonRange', 
        4: 'cannonDamage',
        5: 'reloadSpeed',
        6: 'moveSpeed',
        7: 'turnSpeed',
        8: 'bodyDamage'
      };
      
      const statKey = statKeyMap[keyNumber];
      if (statKey) {
        this.queueAction('statUpgrade', statKey);
      }
      return; // Early return, action already sent
    }
    
    // Handle autofire toggle using new action system
    // queueAction sends immediately, so no need to set inputChanged
    if (e.key === 'r' || e.key === 'R') {
      if (this.queueAction('toggleAutofire', '')) {
        // Optimistic UI update
        if (this.gameState.myPlayer) {
          this.clientAutofireState = !this.gameState.myPlayer.autofireEnabled;
          
          // Clear prediction after 1 second
          if (this.clientAutofireStateTimeout) {
            clearTimeout(this.clientAutofireStateTimeout);
          }
          this.clientAutofireStateTimeout = setTimeout(() => {
            this.clientAutofireState = null;
            this.clientAutofireStateTimeout = null;
          }, 1000);
        }
      }
      return; // Early return, action already sent
    }
    
    // Handle movement keys (continuous state)
    if (e.key === 'w' || e.key === 'W' || e.key === 'ArrowUp') {
      if (!this.input.up) {
        this.input.up = true;
        inputChanged = true;
      }
    }
    if (e.key === 's' || e.key === 'S' || e.key === 'ArrowDown') {
      if (!this.input.down) {
        this.input.down = true;
        inputChanged = true;
      }
    }
    if (e.key === 'a' || e.key === 'A' || e.key === 'ArrowLeft') {
      if (!this.input.left) {
        this.input.left = true;
        inputChanged = true;
      }
    }
    if (e.key === 'd' || e.key === 'D' || e.key === 'ArrowRight') {
      if (!this.input.right) {
        this.input.right = true;
        inputChanged = true;
      }
    }
    
    // Legacy keys (kept for compatibility)
    if (e.key === 'q' || e.key === 'Q') {
      if (!this.input.shootLeft) {
        this.input.shootLeft = true;
        inputChanged = true;
      }
    }
    if (e.key === 'e' || e.key === 'E') {
      if (!this.input.shootRight) {
        this.input.shootRight = true;
        inputChanged = true;
      }
    }
    if (e.key === '=' || e.key === '+') {
      if (!this.input.upgradeCannons) {
        this.input.upgradeCannons = true;
        inputChanged = true;
      }
    }
    if (e.key === '-' || e.key === '_') {
      if (!this.input.downgradeCannons) {
        this.input.downgradeCannons = true;
        inputChanged = true;
      }
    }
    if (e.key === 'p' || e.key === 'P') {
      if (!this.input.upgradeScatter) {
        this.input.upgradeScatter = true;
        inputChanged = true;
      }
    }
    if (e.key === 'o' || e.key === 'O') {
      if (!this.input.downgradeScatter) {
        this.input.downgradeScatter = true;
        inputChanged = true;
      }
    }
    if (e.key === ']' || e.key === '}') {
      if (!this.input.upgradeTurrets) {
        this.input.upgradeTurrets = true;
        inputChanged = true;
      }
    }
    if (e.key === '[' || e.key === '{') {
      if (!this.input.downgradeTurrets) {
        this.input.downgradeTurrets = true;
        inputChanged = true;
      }
    }
    if (e.key === 'l' || e.key === 'L') {
      if (!this.input.debugLevelUp) {
        this.input.debugLevelUp = true;
        inputChanged = true;
      }
    }
    
    if (inputChanged) {
      this.sendInput();
    }
  }

  handleKeyUp(e) {
    if (this.controlsLocked) {
      return;
    }

    let inputChanged = false;
    
    if (e.key === 'w' || e.key === 'W' || e.key === 'ArrowUp') {
      if (this.input.up) {
        this.input.up = false;
        inputChanged = true;
      }
    }
    if (e.key === 's' || e.key === 'S' || e.key === 'ArrowDown') {
      if (this.input.down) {
        this.input.down = false;
        inputChanged = true;
      }
    }
    if (e.key === 'a' || e.key === 'A' || e.key === 'ArrowLeft') {
      if (this.input.left) {
        this.input.left = false;
        inputChanged = true;
      }
    }
    if (e.key === 'd' || e.key === 'D' || e.key === 'ArrowRight') {
      if (this.input.right) {
        this.input.right = false;
        inputChanged = true;
      }
    }
    if (e.key === 'q' || e.key === 'Q') {
      if (this.input.shootLeft) {
        this.input.shootLeft = false;
        inputChanged = true;
      }
    }
    if (e.key === 'e' || e.key === 'E') {
      if (this.input.shootRight) {
        this.input.shootRight = false;
        inputChanged = true;
      }
    }
    if (e.key === '=' || e.key === '+') {
      if (this.input.upgradeCannons) {
        this.input.upgradeCannons = false;
        inputChanged = true;
      }
    }
    if (e.key === '-' || e.key === '_') {
      if (this.input.downgradeCannons) {
        this.input.downgradeCannons = false;
        inputChanged = true;
      }
    }
    if (e.key === 'p' || e.key === 'P') {
      if (this.input.upgradeScatter) {
        this.input.upgradeScatter = false;
        inputChanged = true;
      }
    }
    if (e.key === 'o' || e.key === 'O') {
      if (this.input.downgradeScatter) {
        this.input.downgradeScatter = false;
        inputChanged = true;
      }
    }
    if (e.key === ']' || e.key === '}') {
      if (this.input.upgradeTurrets) {
        this.input.upgradeTurrets = false;
        inputChanged = true;
      }
    }
    if (e.key === '[' || e.key === '{') {
      if (this.input.downgradeTurrets) {
        this.input.downgradeTurrets = false;
        inputChanged = true;
      }
    }
    if (e.key === 'l' || e.key === 'L') {
      if (this.input.debugLevelUp) {
        this.input.debugLevelUp = false;
        inputChanged = true;
      }
    }
    if (e.key === 'r' || e.key === 'R') {
      // Clear pending flag on key release
      this.autofireTogglePending = false;
    }
    
    if (inputChanged) {
      this.sendInput();
    }
  }

  updateMouseWorldCoords() {
    // Convert screen coordinates to world coordinates
    // Account for camera position: camera.x/y represents the top-left corner of the view
    this.input.mouse.x = this.lastMouseScreen.x + this.camera.x;
    this.input.mouse.y = this.lastMouseScreen.y + this.camera.y;
  }

  handleUpgradeUIClick(screenX, screenY) {
    if (this.controlsLocked) {
      return false;
    }

    const player = this.gameState.myPlayer;
    if (!player || player.availableUpgrades <= 0) {
      return false;
    }

    const upgradeTypes = ['side', 'top', 'front', 'rear'];
    const availableTypes = upgradeTypes.filter((type) => this.hasAvailableUpgrades(type));
    if (availableTypes.length === 0) {
      return false;
    }

    const buttonWidth = 50;
    const buttonHeight = 50;
    const spacing = 20;
    const totalWidth = (buttonWidth * availableTypes.length) + (spacing * (availableTypes.length - 1));
    const startX = (this.screenWidth - totalWidth) / 2;
    const buttonY = this.screenHeight - 150;

    for (let i = 0; i < availableTypes.length; i++) {
      const type = availableTypes[i];
      const x = startX + (buttonWidth + spacing) * i;
      if (screenX >= x && screenX <= x + buttonWidth && screenY >= buttonY && screenY <= buttonY + buttonHeight) {
        this.upgradeUI.selectedUpgradeType = this.upgradeUI.selectedUpgradeType === type ? null : type;
        return true;
      }
    }

    const selectedType = this.upgradeUI.selectedUpgradeType;
    if (!selectedType) {
      return false;
    }

    const options = this.getAvailableUpgrades(selectedType);
    if (!options || options.length === 0) {
      return false;
    }

    const typeIndex = availableTypes.indexOf(selectedType);
    if (typeIndex === -1) {
      this.upgradeUI.selectedUpgradeType = null;
      return false;
    }

    const optionHeight = 30;
    const optionWidth = Math.max(buttonWidth, 125);
    const optionSpacing = 10;
    const totalHeight = (optionHeight * options.length) + (optionSpacing * (options.length - 1));
    const optionsX = startX + (buttonWidth + spacing) * typeIndex + (buttonWidth - optionWidth) / 2;
    const optionsStartY = buttonY - totalHeight - 10;

    for (let i = 0; i < options.length; i++) {
      const option = options[i];
      const optionY = optionsStartY + (optionHeight + optionSpacing) * i;

      if (screenX >= optionsX && screenX <= optionsX + optionWidth && screenY >= optionY && screenY <= optionY + optionHeight) {
        console.log(`Selected upgrade: ${selectedType} - ${option.name}`);
        this.selectUpgrade(selectedType, option.name);
        return true;
      }
    }

    return false;
  }
  
  selectUpgrade(upgradeType, upgradeId) {
    // Prevent multiple upgrade selections
    if (this.upgradeUI.pendingUpgrade) {
      console.log('Upgrade selection is already pending; ignoring additional selection.');
      return;
    }
    
    // Mark as pending to prevent duplicate selections
    this.upgradeUI.pendingUpgrade = true;
    this.upgradeUI.upgradeSent = false;
    
    // Set upgrade selection to be sent
    this.input.selectUpgrade = upgradeType;
    this.input.upgradeChoice = upgradeId;
    
    // Clear selected upgrade type to hide the options
    this.upgradeUI.selectedUpgradeType = null;
    
    // Attempt to send immediately
    this.sendUpgradeInput();
    
    // Fallback timeout in case server doesn't respond
    setTimeout(() => {
      // If still pending after timeout, clear everything
      if (this.upgradeUI.pendingUpgrade) {
        console.log('Upgrade timeout - clearing state');
        this.upgradeUI.pendingUpgrade = false;
        this.input.selectUpgrade = '';
        this.input.upgradeChoice = '';
        this.upgradeUI.upgradeSent = false;
      }
    }, 2000); // Longer timeout for server response
  }

  handleStatUpgradeKey(keyNumber) {
    if (!this.gameState.myPlayer || !this.gameState.myPlayer.statUpgrades) return false;
    
    // Check cooldown to prevent spam
    const now = Date.now();
    const lastUpgrade = this.statUpgradeCooldowns[keyNumber] || 0;
    if (now - lastUpgrade < this.statUpgradeCooldownMs) {
      return false; // Still on cooldown
    }
    
    const player = this.gameState.myPlayer;
    
    // Map number keys to stat upgrade types
    const statKeyMap = {
      1: 'hullStrength',
      2: 'autoRepairs',
      3: 'cannonRange', 
      4: 'cannonDamage',
      5: 'reloadSpeed',
      6: 'moveSpeed',
      7: 'turnSpeed',
      8: 'bodyDamage'
    };
    
    const statNames = {
      'hullStrength': 'Hull Strength',
      'autoRepairs': 'Auto Repairs', 
      'cannonRange': 'Cannon Range',
      'cannonDamage': 'Cannon Damage',
      'reloadSpeed': 'Reload Speed',
      'moveSpeed': 'Move Speed',
      'turnSpeed': 'Turn Speed',
      'bodyDamage': 'Body Damage'
    };
    
    const statKey = statKeyMap[keyNumber];
    if (!statKey) return false;
    
    const statUpgrade = player.statUpgrades[statKey];
    if (!statUpgrade) return false;
    
    const level = statUpgrade.level || 0;
    const maxLevel = statUpgrade.maxLevel || 15;
    const cost = statUpgrade.currentCost || 10;
    const coins = player.coins || 0;
    
    // Calculate total upgrades across all stats
    let totalUpgrades = 0;
    Object.values(player.statUpgrades).forEach(upgrade => {
      totalUpgrades += upgrade.level || 0;
    });
    
    if (level >= maxLevel) {
      console.log(`${statNames[statKey]} is already at maximum level (${maxLevel})`);
      return false;
    }
    
    if (totalUpgrades >= 75) {
      console.log(`Cannot upgrade ${statNames[statKey]} - Total upgrade limit reached (75/75)`);
      return false;
    }
    
    if (coins < cost) {
      console.log(`Not enough coins to upgrade ${statNames[statKey]}. Need ${cost}, have ${coins}`);
      return false;
    }
    
    // Update cooldown timestamp
    this.statUpgradeCooldowns[keyNumber] = Date.now();
    
    // Set stat upgrade request (will be sent by main input logic)
    this.input.statUpgradeType = statKey;
    console.log(`Upgrading ${statNames[statKey]} (Level ${level} -> ${level + 1}) for ${cost} coins`);
    return true;
  }

  // Queue an action with deduplication and cooldown checking
  queueAction(actionType, data = '') {
    const now = Date.now();
    
    // Check if there's a pending action of this type
    const pending = this.pendingActions.get(actionType);
    if (pending) {
      const cooldown = this.actionCooldowns[actionType] || 0;
      if (now - pending.timestamp < cooldown) {
        return false; // Still on cooldown
      }
    }
    
    // Create new action with sequence number
    this.actionSequence++;
    const action = {
      type: actionType,
      sequence: this.actionSequence,
      data: data
    };
    
    // Track this action
    this.pendingActions.set(actionType, {
      sequence: this.actionSequence,
      timestamp: now
    });
    
    // Add to actions queue (clear old ones first)
    this.input.actions = this.input.actions.filter(a => a.type !== actionType);
    this.input.actions.push(action);
    
    // Immediately send the input with the queued action
    this.sendInput();
    
    return true;
  }

  sendInput() {
    if (this.controlsLocked) {
      return;
    }
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      return;
    }
    
    // Prevent concurrent sends
    if (this.isSendingInput) {
      return;
    }
    
    // For actions, send immediately. For regular movement, throttle to 30 FPS to match server tick rate
    const now = Date.now();
    const hasActions = this.input.actions.length > 0;
    const timeSinceLastSend = now - this.lastInputSendTime;
    
    if (!hasActions && timeSinceLastSend < 33) {
      // Throttle movement-only updates to 30 FPS (33ms) to match backend tick rate
      return;
    }
    
    this.isSendingInput = true;
    this.lastInputSendTime = now;
    
    try {
      // Send the current input state
      this.socket.send(JSON.stringify(this.input));
      
      // Clear actions after successful send
      this.input.actions = [];
    } finally {
      this.isSendingInput = false;
    }
  }
  
  sendUpgradeInput() {
    // Special send for upgrades that marks them as sent
    if (this.controlsLocked) {
      console.log('Controls locked, cannot send upgrade');
      return;
    }
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      console.log('Socket not ready, cannot send upgrade');
      return;
    }
    
    console.log('Sending upgrade:', this.input.selectUpgrade, this.input.upgradeChoice);
    this.socket.send(JSON.stringify(this.input));
    this.upgradeUI.upgradeSent = true;
  }
  
  sendAutofireToggle() {
    // Special send for autofire toggle
    if (this.controlsLocked) {
      console.log('Controls locked, cannot toggle autofire');
      this.autofireTogglePending = false;
      this.input.toggleAutofire = false;
      return;
    }
    if (!this.socket || this.socket.readyState !== WebSocket.OPEN) {
      console.log('Socket not ready, cannot toggle autofire');
      this.autofireTogglePending = false;
      this.input.toggleAutofire = false;
      return;
    }
    
    console.log('Toggling autofire');
    this.socket.send(JSON.stringify(this.input));
    
    // Clear the toggle flag after sending
    setTimeout(() => {
      this.input.toggleAutofire = false;
      this.autofireTogglePending = false;
    }, 100);
  }

  updateClientPrediction() {
    if (!this.gameState.myPlayer) return;
    
    const currentTime = Date.now();
    const deltaTime = Math.min((currentTime - this.lastPredictionUpdate) / 1000, 1/30); // Cap deltaTime
    this.lastPredictionUpdate = currentTime;
    
    // Ship physics simulation (matching server)
    const physics = this.shipPhysics;
    
    // Handle thrust (W/S keys) - this affects speed, not direction
    let thrustForce = 0;
    if (this.input.up) {
      thrustForce = physics.acceleration;
    }
    if (this.input.down) {
      thrustForce = -physics.acceleration * 0.5; // Reverse is weaker
    }
    
    // Apply thrust in the direction the ship is facing
    if (thrustForce !== 0) {
      const thrustX = Math.cos(physics.angle) * thrustForce;
      const thrustY = Math.sin(physics.angle) * thrustForce;
      physics.velocity.x += thrustX;
      physics.velocity.y += thrustY;
    }
    
    // Calculate current speed for turn scaling (matching server logic)
    const speed = Math.min(Math.sqrt(physics.velocity.x * physics.velocity.x + physics.velocity.y * physics.velocity.y), physics.maxSpeed);
    
    // Scale turn speed based on current speed (matching server logic)
    let turnFactor = speed / physics.maxSpeed;
    const scaledTurnSpeed = physics.turnSpeed * turnFactor;
    
    // Handle turning (A/D keys) with speed-based scaling
    if (this.input.left) {
      physics.angle -= scaledTurnSpeed;
    }
    if (this.input.right) {
      physics.angle += scaledTurnSpeed;
    }
    
    // Apply drag/deceleration
    physics.velocity.x *= physics.deceleration;
    physics.velocity.y *= physics.deceleration;
    
    // Limit maximum speed
    const newSpeed = Math.sqrt(physics.velocity.x * physics.velocity.x + physics.velocity.y * physics.velocity.y);
    if (newSpeed > physics.maxSpeed) {
      const speedRatio = physics.maxSpeed / newSpeed;
      physics.velocity.x *= speedRatio;
      physics.velocity.y *= speedRatio;
    }
    
    // Update predicted position with delta time scaling for 30 TPS
    const moveX = physics.velocity.x * deltaTime * 30; // Matches server physics
    const moveY = physics.velocity.y * deltaTime * 30;
    
    this.predictedPlayerPos.x += moveX;
    this.predictedPlayerPos.y += moveY;
    
    // Keep within world bounds
    if (this.predictedPlayerPos.x <= 10) {
      this.predictedPlayerPos.x = 10;
      physics.velocity.x = Math.max(0, physics.velocity.x); // Stop negative velocity
    }
    if (this.predictedPlayerPos.x >= WorldWidth - 10) {
      this.predictedPlayerPos.x = WorldWidth - 10;
      physics.velocity.x = Math.min(0, physics.velocity.x); // Stop positive velocity
    }
    if (this.predictedPlayerPos.y <= 10) {
      this.predictedPlayerPos.y = 10;
      physics.velocity.y = Math.max(0, physics.velocity.y);
    }
    if (this.predictedPlayerPos.y >= WorldHeight - 10) {
      this.predictedPlayerPos.y = WorldHeight - 10;
      physics.velocity.y = Math.min(0, physics.velocity.y);
    }
  }

  updateCamera() {
    if (this.gameState.myPlayer) {
      // Store previous camera position to detect changes
      const prevCameraX = this.camera.x;
      const prevCameraY = this.camera.y;
      
      // Use server position for camera to avoid jitter
      // this.camera.targetX = this.predictedPlayerPos.x - this.screenWidth / 2;
      // this.camera.targetY = this.predictedPlayerPos.y - this.screenHeight / 2;
      this.camera.targetX = this.gameState.myPlayer.x - this.screenWidth / 2;
      this.camera.targetY = this.gameState.myPlayer.y - this.screenHeight / 2;
      
      // Smooth camera movement
      const cameraLerpFactor = 1;
      this.camera.x += (this.camera.targetX - this.camera.x) * cameraLerpFactor;
      this.camera.y += (this.camera.targetY - this.camera.y) * cameraLerpFactor;
      
      // Check if camera position changed and update mouse world coordinates
      if (prevCameraX !== this.camera.x || prevCameraY !== this.camera.y) {
        this.updateMouseWorldCoords();
        // Send updated input with new mouse world position
        this.sendInput();
      }
    }
  }

  render() {
    // Clear canvas
    this.ctx.fillStyle = '#9bbfeaff';
    this.ctx.fillRect(0, 0, this.screenWidth, this.screenHeight);
    
    if (!this.isConnected) {
      this.ctx.fillStyle = '#ff6b6b';
      this.ctx.font = '24px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText('Connecting...', this.screenWidth / 2, this.screenHeight / 2);
      return;
    }

    this.updateClientPrediction();
    this.updateCamera();

    // Draw world grid
    this.drawGrid();
    
    // Draw map border
    this.drawMapBorder();
    
    // Draw items
    this.gameState.items.forEach(item => {
      this.drawItem(item);
    });
    
    // Draw bullets
    this.gameState.bullets.forEach(bullet => {
      this.drawBullet(bullet);
    });
    
    // Draw players
    this.gameState.players.forEach(player => {
      this.drawPlayer(player);
    });
    
    // Draw UI
    this.drawUI();

    // Draw debug status
    this.drawDebugStatus();
    
    // Draw death screen on top of everything
    this.drawDeathScreen();
  }

  drawGrid() {
    const gridSize = 50; // Larger grid for better performance
    this.ctx.strokeStyle = '#808080';
    this.ctx.lineWidth = 1;
    
    const startX = Math.floor(this.camera.x / gridSize) * gridSize;
    const startY = Math.floor(this.camera.y / gridSize) * gridSize;
    
    // Draw fewer grid lines by using larger steps
    this.ctx.beginPath();
    for (let x = startX; x < this.camera.x + this.screenWidth + gridSize; x += gridSize) {
      this.ctx.moveTo(x - this.camera.x, 0);
      this.ctx.lineTo(x - this.camera.x, this.screenHeight);
    }
    
    for (let y = startY; y < this.camera.y + this.screenHeight + gridSize; y += gridSize) {
      this.ctx.moveTo(0, y - this.camera.y);
      this.ctx.lineTo(this.screenWidth, y - this.camera.y);
    }
    this.ctx.stroke();
  }

  drawMapBorder() {
    // Convert world coordinates to screen coordinates
    const borderLeft = 0 - this.camera.x;
    const borderTop = 0 - this.camera.y;
    const borderRight = WorldWidth - this.camera.x;
    const borderBottom = WorldHeight - this.camera.y;
    
    // Only draw border segments that are visible on screen
    this.ctx.strokeStyle = '#404040'; 
    this.ctx.lineWidth = 4;
    
    this.ctx.beginPath();
    
    // Top border
    if (borderTop >= -4 && borderTop <= this.screenHeight + 4) {
      this.ctx.moveTo(Math.max(0, borderLeft), borderTop);
      this.ctx.lineTo(Math.min(this.screenWidth, borderRight), borderTop);
    }
    
    // Bottom border
    if (borderBottom >= -4 && borderBottom <= this.screenHeight + 4) {
      this.ctx.moveTo(Math.max(0, borderLeft), borderBottom);
      this.ctx.lineTo(Math.min(this.screenWidth, borderRight), borderBottom);
    }
    
    // Left border
    if (borderLeft >= -4 && borderLeft <= this.screenWidth + 4) {
      this.ctx.moveTo(borderLeft, Math.max(0, borderTop));
      this.ctx.lineTo(borderLeft, Math.min(this.screenHeight, borderBottom));
    }
    
    // Right border
    if (borderRight >= -4 && borderRight <= this.screenWidth + 4) {
      this.ctx.moveTo(borderRight, Math.max(0, borderTop));
      this.ctx.lineTo(borderRight, Math.min(this.screenHeight, borderBottom));
    }
    
    this.ctx.stroke();
  }

drawPlayer(player) {
  if (player.state !== 0) {
    return; // Skip rendering players that are not alive
  }
  const ctx = this.ctx;
  const screenX = player.x - this.camera.x;
  const screenY = player.y - this.camera.y;

  const size = player.size || 50;
  const color = player.color || '#d9534f';
  const angle = player.angle || 0;

  // --- Ship dimensions from backend ---
  const bowLength = size * 0.4;
  const shaftLength = player.shipConfig.shipLength || size * 1.2; // Backend provides the shaft length directly
  const rearLength = size * 0.3;
  const shaftWidth = player.shipConfig.shipWidth || size * 0.6;
  const totalRearLength = rearLength;

  // --- Cannon rendering dimensions ---
  const gunLength = size * 0.35;
  const gunWidth = size * 0.2;

  ctx.save();
  ctx.translate(screenX, screenY);
  ctx.rotate(angle);

  // --- Draw cannons and turrets first (under the ship) ---
  ctx.fillStyle = '#666';
  ctx.strokeStyle = '#333';
  ctx.lineWidth = 2;

  // Draw side cannons from modular system
  if (player.shipConfig && player.shipConfig.sideUpgrade && player.shipConfig.sideUpgrade.cannons) {
    for (const cannon of player.shipConfig.sideUpgrade.cannons) {
      // Backend provides relative positions, draw cannon centered on that position
      let centerX = cannon.position.x;
      let centerY = cannon.position.y;
      
      // Calculate recoil animation offset
      if (cannon.recoilTime) {
        const timeSinceFire = Date.now() - new Date(cannon.recoilTime).getTime();
        const recoilDuration = 400; // 200ms recoil animation
        
        if (timeSinceFire < recoilDuration) {
          const progress = timeSinceFire / recoilDuration;
          // Ease-out animation: starts fast, slows down
          const easeOut = 1 - Math.pow(1 - progress, 3);
          const recoilDistance = 8; // Maximum recoil distance in pixels
          
          // Calculate recoil offset (side cannons recoil horizontally inward)
          const recoilOffset = recoilDistance * (1 - easeOut);
          // Side cannons recoil perpendicular to ship side (toward ship centerline)
          // Determine if cannon is on left or right side and recoil accordingly
          if (centerY > 0) {
            // Right side cannon - recoil leftward (negative Y)
            centerY -= recoilOffset;
          } else {
            // Left side cannon - recoil rightward (positive Y)
            centerY += recoilOffset;
          }
        }
      }
      
      if (cannon.type === 'row') {
        // Draw rowing oar as animated thin gray rectangle
        ctx.fillStyle = '#666'; // Gray color for oars
        ctx.strokeStyle = '#333'; // Darker gray for outline
        
        // Animation timing
        const time = Date.now() / 1000; // Convert to seconds
        
        // Calculate ship's current speed
        const shipSpeed = Math.sqrt(player.velX * player.velX + player.velY * player.velY);
        
        const rowSpeed = 1.0

        const maxAngle = Math.PI / 6; // 30 degrees max rowing angle
        
        // Calculate rowing angle (sinusoidal motion)
        const rowAngle = Math.sin(time * rowSpeed * Math.PI * 2) * maxAngle;
        
        // Determine if this is a right or left side oar
        const isRightSide = centerY > 0;
        const appliedRowAngle = isRightSide ? rowAngle : -rowAngle;
        
        // Make oars thinner than cannons
        const oarLength = gunLength*1.2;
        const oarWidth = gunWidth; // Half the width of regular cannons
        
        ctx.save();
        ctx.translate(centerX, centerY);
        // Base rotation: 90 degrees to make oars perpendicular to ship side
        // Then add rowing animation on top
        ctx.rotate(Math.PI / 2 + appliedRowAngle);
        
        // Draw thin oar rectangle
        const x = -oarLength / 2;
        const y = -oarWidth / 2;
        ctx.fillRect(x, y, oarLength, oarWidth);
        ctx.strokeRect(x, y, oarLength, oarWidth);
        
        ctx.restore();
      } else if (cannon.type === 'scatter') {
        // Draw scatter cannon as a trapezoid with wider base facing away from ship
        const baseWidth = gunWidth * 2;   // Narrower base (along ship side)
        const muzzleWidth = gunWidth * 3;  // Wider muzzle (facing outward)
        
        // Determine if this is a left or right side cannon based on Y position
        const isRightSide = centerY > 0;
        
        ctx.beginPath();
        if (isRightSide) {
          // Right side cannon - trapezoid with muzzle facing away from ship (positive Y)
          // Back-inner corner (narrow end, closer to ship center)
          ctx.moveTo(centerX - baseWidth/2, centerY - gunWidth/2);
          // Front-inner corner (narrow end)
          ctx.lineTo(centerX + baseWidth/2, centerY - gunWidth/2);
          // Front-outer corner (wide end, muzzle)
          ctx.lineTo(centerX + muzzleWidth/2, centerY + gunWidth/2);
          // Back-outer corner (wide end)
          ctx.lineTo(centerX - muzzleWidth/2, centerY + gunWidth/2);
        } else {
          // Left side cannon - trapezoid with muzzle facing away from ship (negative Y)
          // Back-inner corner (narrow end, closer to ship center)
          ctx.moveTo(centerX - baseWidth/2, centerY + gunWidth/2);
          // Front-inner corner (narrow end)
          ctx.lineTo(centerX + baseWidth/2, centerY + gunWidth/2);
          // Front-outer corner (wide end, muzzle)
          ctx.lineTo(centerX + muzzleWidth/2, centerY - gunWidth/2);
          // Back-outer corner (wide end)
          ctx.lineTo(centerX - muzzleWidth/2, centerY - gunWidth/2);
        }
        // Close the shape
        ctx.closePath();
        ctx.fill();
        
        // Add stroke for better visibility
        ctx.strokeStyle = '#333';
        ctx.stroke();
      } else {
        // Draw regular cannon as rectangle
        const x = centerX - gunLength / 2; // Convert center to top-left for fillRect
        const y = centerY - gunWidth / 2;  // Convert center to top-left for fillRect
        ctx.fillRect(x, y, gunLength, gunWidth);
        ctx.strokeRect(x, y, gunLength, gunWidth);
      }
    }
  }

  // --- Draw Ram upgrade (gray triangle on front) ---
  if (player.shipConfig && player.shipConfig.frontUpgrade) {
    if (player.shipConfig.frontUpgrade.name == 'Ram') {
      const ramLength = bowLength * 1;
      const ramWidth = shaftWidth * 0.6;
      
      ctx.fillStyle = '#444'; // Gray color for ram
      
      ctx.beginPath();
      ctx.moveTo(shaftLength / 2 + bowLength - 8 + ramLength, 0); // ram tip
      ctx.lineTo(shaftLength / 2 + bowLength - 8, ramWidth / 2);  // ram base right
      ctx.lineTo(shaftLength / 2 + bowLength - 8, -ramWidth / 2); // ram base left
      ctx.closePath();
      ctx.fill();
    }
    else {
      for (const cannon of player.shipConfig.frontUpgrade.cannons || []) {
        // Draw front cannons similarly to side cannons
        let centerX = cannon.position.x;
        let centerY = cannon.position.y;
        
        // Calculate recoil animation offset
        if (cannon.recoilTime) {
          const timeSinceFire = Date.now() - new Date(cannon.recoilTime).getTime();
          const recoilDuration = 400; // 200ms recoil animation
          
          if (timeSinceFire < recoilDuration) {
            const progress = timeSinceFire / recoilDuration;
            // Ease-out animation: starts fast, slows down
            const easeOut = 1 - Math.pow(1 - progress, 3);
            const recoilDistance = 8; // Maximum recoil distance in pixels
            
            // Front cannons recoil backward along ship's forward axis
            const recoilOffset = recoilDistance * (1 - easeOut);
            centerX -= recoilOffset; // Move backward along X axis
          }
        }
        
        // Draw regular cannon as rectangle
        const x = centerX - gunLength / 2; // Convert center to top-left for fillRect
        const y = centerY - gunWidth / 2;  // Convert center to top-left for fillRect
        ctx.fillRect(x, y, gunLength, gunWidth);
        ctx.strokeRect(x, y, gunLength, gunWidth);
      }
    }
  }

  if (player.shipConfig && player.shipConfig.rearUpgrade) {
    if (player.shipConfig.rearUpgrade.name == 'Rudder') {
      const rudderLength = size * 0.25;
      const rudderWidth = shaftWidth * 0.3;
      
      ctx.fillStyle = '#666'; // Gray color for rectangular rudder

      ctx.beginPath();
      ctx.moveTo(-shaftLength / 2 - totalRearLength - rudderLength, rudderWidth / 2);
      ctx.lineTo(-shaftLength / 2 - totalRearLength , rudderWidth / 2);
      ctx.lineTo(-shaftLength / 2 - totalRearLength, -rudderWidth / 2);
      ctx.lineTo(-shaftLength / 2 - totalRearLength - rudderLength, -rudderWidth / 2);
      ctx.closePath();
      ctx.fill();
    }
  }

  ctx.fillStyle = color;
  ctx.strokeStyle = '#444';
  ctx.lineWidth = 3;
  

  // --- Draw main hull ---
  ctx.beginPath();

  ctx.moveTo(shaftLength / 2 + bowLength, 0); // bow tip

  ctx.quadraticCurveTo(
    shaftLength / 2 + bowLength * 0.3,
    shaftWidth / 2,
    shaftLength / 2,
    shaftWidth / 2
  );
  ctx.lineTo(-shaftLength / 2, shaftWidth / 2);
  ctx.lineTo(-shaftLength / 2 - totalRearLength, shaftWidth / 2 * 0.5);
  ctx.lineTo(-shaftLength / 2 - totalRearLength, -shaftWidth / 2 * 0.5);
  ctx.lineTo(-shaftLength / 2, -shaftWidth / 2);
  ctx.lineTo(shaftLength / 2, -shaftWidth / 2);
  ctx.quadraticCurveTo(
    shaftLength / 2 + bowLength * 0.3,
    -shaftWidth / 2,
    shaftLength / 2 + bowLength,
    0
  );
  ctx.closePath();
  ctx.fill();
  ctx.stroke();



  // --- Center circle ---
  if (player.shipConfig && player.shipConfig.topUpgrade && player.shipConfig.topUpgrade.turrets && player.shipConfig.topUpgrade.turrets.length == 0 || !player.shipConfig.topUpgrade.turrets) {
    ctx.beginPath();
    ctx.arc(0, 0, shaftWidth * 0.2, 0, Math.PI * 2);
    ctx.fillStyle = '#444';
    ctx.fill();
    ctx.strokeStyle = '#444';
    ctx.stroke();
  }
  

  // --- Draw cannons using new modular system ---
  ctx.fillStyle = '#666';
  // --- Draw turrets using new modular system ---
  if (player.shipConfig && player.shipConfig.topUpgrade && player.shipConfig.topUpgrade.turrets) {
    for (const turret of player.shipConfig.topUpgrade.turrets) {
      // Draw turret base (circular mount)
      const turretSize = size * 0.5;
      const barrelLength = size * 0.5;
      const barrelWidth = size * 0.2;
      let baseSize = turretSize * 0.5;
      
      ctx.save();
      ctx.translate(turret.position.x, turret.position.y);

      // Draw turret barrel(s) based on turret type
      ctx.rotate(turret.angle - angle);
      ctx.fillStyle = '#666';
      
      // Calculate recoil offset for turret barrels
      let recoilOffset = 0;
      if (turret.recoilTime) {
        const timeSinceFire = Date.now() - new Date(turret.recoilTime).getTime();
        const recoilDuration = 200; // 200ms recoil animation
        
        if (timeSinceFire < recoilDuration) {
          const progress = timeSinceFire / recoilDuration;
          // Ease-out animation: starts fast, slows down
          const easeOut = 1 - Math.pow(1 - progress, 3);
          const recoilDistance = 6; // Maximum recoil distance in pixels
          recoilOffset = -recoilDistance * (1 - easeOut); // Negative for backward recoil
        }
      }
      
      if (turret.type === 'machine_gun_turret' && turret.cannons && turret.cannons.length >= 2) {
        // Draw two parallel barrels for machine gun turret with alternating recoil
        const barrelSeparation = barrelWidth;
        const numCannons = turret.cannons.length;
        
        // Determine which cannon just fired (previous to nextCannonIndex)
        const lastFiredIndex = (turret.nextCannonIndex - 1 + numCannons) % numCannons;
        
        // Left barrel (cannon index 0)
        const leftRecoil = (lastFiredIndex === 0) ? recoilOffset : 0;
        ctx.fillRect(leftRecoil, -barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        ctx.strokeRect(leftRecoil, -barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        
        // Right barrel (cannon index 1)
        const rightRecoil = (lastFiredIndex === 1) ? recoilOffset : 0;
        ctx.fillRect(rightRecoil, barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        ctx.strokeRect(rightRecoil, barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        baseSize = turretSize * 0.6;
      } 
      else if (turret.type === 'big_turret') {
        const bigBarrelWidth = barrelWidth * 2;

        // Proportions
        const baseLength = barrelLength*1.2;
        const muzzleLength = barrelLength * 0.5;
        const baseWidthBack = bigBarrelWidth * 1.5; // slightly wider at the turret
        const baseWidthFront = bigBarrelWidth * 1; // slightly narrower where it meets the barrel

        // --- Draw trapezoidal base (wide at turret, narrower toward barrel) ---
        ctx.beginPath();
        ctx.moveTo(recoilOffset, -baseWidthBack / 2); // top back (turret side)
        ctx.lineTo(recoilOffset + baseLength, -baseWidthFront / 2); // top front (joins barrel)
        ctx.lineTo(recoilOffset + baseLength, baseWidthFront / 2);  // bottom front
        ctx.lineTo(recoilOffset, baseWidthBack / 2); // bottom back
        ctx.closePath();
        ctx.fillStyle = '#666';
        ctx.fill();
        ctx.stroke();

        // --- Draw main rectangular barrel extending forward from trapezoid ---
        ctx.fillStyle = '#666';
        ctx.fillRect(
          recoilOffset + baseLength,           // start right after trapezoid
          -baseWidthFront / 2,                 // vertically centered
          muzzleLength,                          // length of rectangular part
          baseWidthFront                       // width matches trapezoid tip
        );
        ctx.strokeRect(
          recoilOffset + baseLength,
          -baseWidthFront / 2,
          muzzleLength,
          baseWidthFront
        );

        baseSize = turretSize * 0.7;
      }
      else {
        // Single barrel for regular turret
        ctx.fillRect(recoilOffset, -barrelWidth / 2, barrelLength, barrelWidth);
        ctx.strokeRect(recoilOffset, -barrelWidth / 2, barrelLength, barrelWidth);
        baseSize = turretSize * 0.5;
      }

      
      // Draw turret base (slightly larger for machine gun turrets)
      ctx.fillStyle = '#666';
      ctx.beginPath();
      ctx.arc(0, 0, baseSize, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      
      ctx.restore();
    }
  }

  ctx.restore();
  
  // Calculate total stat upgrade level
  let totalUpgrades = 0;
  if (player.statUpgrades) {
    Object.values(player.statUpgrades).forEach(upgrade => {
      totalUpgrades += upgrade.level || 0;
    });
  }
  
  // Draw player name with upgrade level prefix above the ship
  const displayName = (player.name && player.name.trim()) ? player.name.trim() : `Player ${player.id}`;
  const labelY = screenY - (shaftWidth / 2) - 20;

  this.ctx.save();
  this.ctx.lineWidth = 3;
  this.ctx.strokeStyle = 'rgba(15, 15, 35, 0.65)';
  this.ctx.fillStyle = player.id === this.myPlayerId ? '#FFFFFF' : '#D7D7D7';
  
  // Measure both text elements to center them together
  this.ctx.font = 'bold 22px Arial';
  const levelWidth = this.ctx.measureText(totalUpgrades.toString()).width;
  this.ctx.font = 'bold 18px Arial';
  const nameWidth = this.ctx.measureText(displayName).width;
  const spacing = 5;
  const totalWidth = levelWidth + spacing + nameWidth;
  
  // Calculate starting X position to center the combined text
  const startX = screenX - totalWidth / 2;
  
  // Draw the upgrade level number (larger, on the left)
  this.ctx.font = 'bold 22px Arial';
  this.ctx.textAlign = 'left';
  this.ctx.strokeText(totalUpgrades.toString(), startX, labelY);
  this.ctx.fillText(totalUpgrades.toString(), startX, labelY);
  
  // Draw the player name (smaller, to the right)
  this.ctx.font = 'bold 18px Arial';
  const nameX = startX + levelWidth + spacing;
  this.ctx.strokeText(displayName, nameX, labelY);
  this.ctx.fillText(displayName, nameX, labelY);
  
  this.ctx.restore();
  
  // Draw health bar above the ship
  this.drawHealthBar(player, screenX, screenY);
}

  drawHealthBar(player, screenX, screenY) {
    const ctx = this.ctx;
    const maxHealth = player.maxHealth || 100;
    const currentHealth = player.health || maxHealth;
    const healthPercentage = currentHealth / maxHealth;
    
    // Health bar dimensions - width scales with max health
    const baseWidth = 50;
    const barWidth = Math.max(baseWidth, baseWidth + (maxHealth - 100) * 0.05); // Wider for higher max health
    const barHeight = 8;
    const barOffsetY = 50; // Position above the ship
    const borderRadius = 4; // Rounded corners
    
    // Skip drawing if player is dead
    if (currentHealth <= 0) {
      return;
    }
    
    ctx.save();
    
    // Health bar background (dark red/gray rounded rectangle)
    ctx.fillStyle = '#444444';
    this.drawRoundedRect(screenX - barWidth/2, screenY + barOffsetY, barWidth, barHeight, borderRadius);
    ctx.fill();
    
    // Health bar foreground - green for own player, red for enemies
    const isOwnPlayer = player.id === this.myPlayerId;
    const healthColor = isOwnPlayer ? '#00cc00' : '#d9534f'; // Green for self, red for enemies
    
    ctx.fillStyle = healthColor;
    const fillWidth = barWidth * healthPercentage;
    if (fillWidth > 0) {
      this.drawRoundedRect(screenX - barWidth/2, screenY + barOffsetY, fillWidth, barHeight, borderRadius);
      ctx.fill();
    }
    
    // Health bar borderw
    ctx.strokeStyle = '#444444';
    ctx.lineWidth = 2;
    this.drawRoundedRect(screenX - barWidth/2, screenY + barOffsetY, barWidth, barHeight, borderRadius);
    ctx.stroke();
    
    ctx.restore();
  }

  drawItem(item) {
    const screenX = item.x - this.camera.x;
    const screenY = item.y - this.camera.y;
    
    // Skip if item is off screen
    if (screenX < -20 || screenX > this.screenWidth + 20 ||
        screenY < -20 || screenY > this.screenHeight + 20) {
      return;
    }
    
    let color = '#808080'; // Default gray
    let size = 7;
    let shape = 'circle';
    
    switch (item.type) {
      case 'gray_circle':
        color = '#808080'; // Gray
        shape = 'circle';
        break;
      case 'yellow_circle':
        color = '#FFD700'; // Yellow/Gold
        shape = 'circle';
        break;
      case 'orange_circle':
        color = '#FF8C00'; // Orange
        shape = 'circle';
        break;
      case 'blue_diamond':
        color = '#4169E1'; // Royal Blue
        size = 14;
        shape = 'diamond';
        break;
      // Legacy support for old item types
      case 'coin':
        color = '#FFD700';
        size = 8;
        shape = 'circle';
        break;
      case 'food':
        color = '#808080';
        size = 8;
        shape = 'circle';
        break;
      case 'health_pack':
        color = '#FF6B6B';
        size = 10;
        shape = 'cross';
        break;
      case 'size_boost':
        color = '#96CEB4';
        size = 12;
        shape = 'circle';
        break;
      case 'speed_boost':
        color = '#FFEAA7';
        size = 10;
        shape = 'diamond';
        break;
      case 'score_multiplier':
        color = '#DDA0DD';
        size = 14;
        shape = 'star';
        break;
    }
    
    // Draw the item shape
    this.ctx.fillStyle = color;
    
    if (shape === 'circle') {
      this.ctx.beginPath();
      this.ctx.arc(screenX, screenY, size, 0, Math.PI * 2);
      this.ctx.fill();
      
      // Add outline based on item value
      this.ctx.strokeStyle = this.getItemOutlineColor(item.type);
      this.ctx.lineWidth = this.getItemOutlineWidth(item.type);
      this.ctx.stroke();
      
    } else if (shape === 'cross') {
      // Draw a cross for health packs
      this.ctx.fillRect(screenX - size/2, screenY - size/6, size, size/3);
      this.ctx.fillRect(screenX - size/6, screenY - size/2, size/3, size);
      
      // Add outline
      this.ctx.strokeStyle = '#ffffff';
      this.ctx.lineWidth = 1;
      this.ctx.strokeRect(screenX - size/2, screenY - size/6, size, size/3);
      this.ctx.strokeRect(screenX - size/6, screenY - size/2, size/3, size);
      
    } else if (shape === 'diamond') {
      // Draw a diamond (for blue_diamond and speed_boost)
      this.ctx.beginPath();
      this.ctx.moveTo(screenX, screenY - size);
      this.ctx.lineTo(screenX + size * 0.7, screenY);
      this.ctx.lineTo(screenX, screenY + size);
      this.ctx.lineTo(screenX - size * 0.7, screenY);
      this.ctx.closePath();
      this.ctx.fill();
      
      // Add special outline for blue diamond
      if (item.type === 'blue_diamond') {
        this.ctx.strokeStyle = '#87CEEB'; // Light blue outline
        this.ctx.lineWidth = 2;
        this.ctx.stroke();
        
        // Add inner glow effect
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 1;
        this.ctx.stroke();
      } else {
        this.ctx.strokeStyle = '#ffffff';
        this.ctx.lineWidth = 1;
        this.ctx.stroke();
      }
      
    } else if (shape === 'star') {
      // Draw a simple star for score multiplier
      this.ctx.beginPath();
      for (let i = 0; i < 5; i++) {
        const angle = (i * 2 * Math.PI) / 5 - Math.PI / 2;
        const x = screenX + Math.cos(angle) * size;
        const y = screenY + Math.sin(angle) * size;
        if (i === 0) this.ctx.moveTo(x, y);
        else this.ctx.lineTo(x, y);
      }
      this.ctx.closePath();
      this.ctx.fill();
      
      // Add outline
      this.ctx.strokeStyle = '#ffffff';
      this.ctx.lineWidth = 1;
      this.ctx.stroke();
    }
  }

  // Helper function to get outline color based on item type
  getItemOutlineColor(itemType) {
    switch (itemType) {
      case 'gray_circle':
        return '#A0A0A0'; // Light gray outline
      case 'yellow_circle':
        return '#FFF700'; // Bright yellow outline
      case 'orange_circle':
        return '#FFA500'; // Bright orange outline
      case 'blue_diamond':
        return '#87CEEB'; // Light blue outline
      default:
        return '#ffffff'; // Default white outline
    }
  }

  // Helper function to get outline width based on item type
  getItemOutlineWidth(itemType) {
    switch (itemType) {
      case 'gray_circle':
        return 1; // Thin outline for common items
      case 'yellow_circle':
        return 1.5; // Medium outline
      case 'orange_circle':
        return 2; // Thicker outline for uncommon items
      case 'blue_diamond':
        return 2.5; // Thickest outline for rare items
      default:
        return 1; // Default thin outline
    }
  }

  drawBullet(bullet) {
    const screenX = bullet.x - this.camera.x;
    const screenY = bullet.y - this.camera.y;
    
    // Only draw if bullet is on screen
    if (screenX < -50 || screenX > this.screenWidth + 50 || 
        screenY < -50 || screenY > this.screenHeight + 50) {
      return;
    }
    
    this.ctx.save();
    this.ctx.translate(screenX, screenY);
    
    // Draw bullet as a bright orange/yellow circle
    this.ctx.beginPath();
    this.ctx.arc(0, 0, bullet.size, 0, Math.PI * 2);
    this.ctx.fillStyle = '#484848ff'; // Gold color for bullets
    this.ctx.fill();
    
    // Add a bright outline
    this.ctx.strokeStyle = '#2a2a2aff'; // Orange outline
    this.ctx.lineWidth = 1;
    this.ctx.stroke();
    
    this.ctx.restore();
  }

  drawUI() {
    // Draw stat upgrade panel (moved to top left)
    this.drawStatUpgradePanel();
    
    // Mini leaderboard in top right
    this.drawLeaderboard();
    
    // Draw minimap
    this.drawMinimap();
    
    // Draw level progress bar
    this.drawLevelProgressBar();
    
    // Draw upgrade UI
    this.drawUpgradeUI();
    
    // Draw autofire status (bottom left)
    this.drawAutofireStatus();

    this.drawKillNotifications();
  }

  drawKillNotifications() {
    const now = Date.now();
    this.killNotifications = this.killNotifications.filter(notification => notification.expiresAt > now);
    if (this.killNotifications.length === 0) {
      return;
    }

    const ctx = this.ctx;
    ctx.save();
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.font = 'bold 24px Arial';

    const startY = Math.max(90, this.screenHeight * 0.18);
    const spacing = 44;

    this.killNotifications.forEach((notification, index) => {
      const remaining = notification.expiresAt - now;
      const fadeWindow = Math.min(500, notification.duration);
      const opacity = fadeWindow > 0 && remaining < fadeWindow ? remaining / fadeWindow : 1;

      const x = this.screenWidth / 2;
      const y = startY + index * spacing;
      const padding = 18;
      const textWidth = ctx.measureText(notification.message).width;
      const boxWidth = textWidth + padding * 2;
      const boxHeight = 38;
      const boxX = x - boxWidth / 2;
      const boxY = y - boxHeight / 2;

      ctx.fillStyle = `rgba(16, 16, 16, ${0.7 * opacity})`;
      this.drawRoundedRect(boxX, boxY, boxWidth, boxHeight, 10);
      ctx.fill();

      ctx.fillStyle = `rgba(255, 255, 255, ${opacity})`;
      ctx.fillText(notification.message, x, y);
    });

    ctx.restore();
  }

  drawStatUpgradePanel() {
    if (!this.gameState.myPlayer) return;
    
    const player = this.gameState.myPlayer;
    const panelX = 10;
    const panelY = 10; // Top left corner
    const panelWidth = 250;
    
    // Calculate total upgrades for the counter
    let totalUpgrades = 0;
    if (player.statUpgrades) {
      Object.values(player.statUpgrades).forEach(upgrade => {
        totalUpgrades += upgrade.level || 0;
      });
    }
    
    // Title bar with background
    const titleBarHeight = 25;
    const titleBarY = panelY + 5;
    
    // Title background
    this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
    this.drawRoundedRect(panelX, titleBarY, panelWidth, titleBarHeight, 5);
    this.ctx.fill();
    
    // Title border
    this.ctx.strokeStyle = 'rgba(64, 64, 64, 0.8)';
    this.ctx.lineWidth = 2;
    this.drawRoundedRect(panelX, titleBarY, panelWidth, titleBarHeight, 5);
    this.ctx.stroke();
    
    // Title text (centered)
    this.ctx.fillStyle = '#FFFFFF';
    this.ctx.font = 'bold 16px Arial';
    this.ctx.textAlign = 'center';
    this.ctx.fillText(`Stat Upgrades ${totalUpgrades}/75`, panelX + panelWidth / 2, titleBarY + 18);
    
    // Coins bar with background
    const coinsBarHeight = 25;
    const coinsBarY = titleBarY + titleBarHeight + 5;
    
    // Coins background
    this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
    this.drawRoundedRect(panelX, coinsBarY, panelWidth, coinsBarHeight, 5);
    this.ctx.fill();
    
    // Coins border
    this.ctx.strokeStyle = 'rgba(64, 64, 64, 0.8)';
    this.ctx.lineWidth = 2;
    this.drawRoundedRect(panelX, coinsBarY, panelWidth, coinsBarHeight, 5);
    this.ctx.stroke();
    
    // Coins text (centered)
    this.ctx.fillStyle = '#FFFFFF';
    this.ctx.font = 'bold 14px Arial';
    this.ctx.textAlign = 'center';
    this.ctx.fillText(`$ Coins: ${player.coins || 0}`, panelX + panelWidth / 2, coinsBarY + 18);
    
    // Stat upgrades
    if (player.statUpgrades) {
      const statNames = {
        'hullStrength': 'Hull Strength',
        'autoRepairs': 'Auto Repairs', 
        'cannonRange': 'Cannon Range',
        'cannonDamage': 'Cannon Damage',
        'reloadSpeed': 'Reload Speed',
        'moveSpeed': 'Move Speed',
        'turnSpeed': 'Turn Speed',
        'bodyDamage': 'Body Damage'
      };
      
      let yOffset = coinsBarY + coinsBarHeight ; // Start after coins bar
      const statOrder = [
        'hullStrength', 'autoRepairs', 'cannonRange', 'cannonDamage',
        'reloadSpeed', 'moveSpeed', 'turnSpeed', 'bodyDamage'
      ];
      
      statOrder.forEach((statKey, index) => {
        const statUpgrade = player.statUpgrades[statKey];
        if (statUpgrade) {
          const level = statUpgrade.level || 0;
          const maxLevel = 15;
          const cost = statUpgrade.currentCost || 10;
          const keyNumber = index + 1;
          
          // Individual upgrade bar with padding
          const barX = panelX;
          const barY = panelY + yOffset;
          const barWidth = panelWidth; // Leave space for key number on the right
          const barHeight = 20;
          const borderRadius = 5;
          
          // Background (empty part)
          this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
          this.drawRoundedRect(barX, barY, barWidth, barHeight, borderRadius);
          this.ctx.fill();
          
          // Progress fill
          const progress = level / maxLevel;
          const fillWidth = barWidth * progress;
          
          if (level > 0) {
            this.ctx.fillStyle = '#B0B0B0'; // Light gray for upgraded
          } else {
            this.ctx.fillStyle = '#606060'; // Medium gray for no upgrades
          }
          
          if (fillWidth > 0) {
            this.drawRoundedRect(barX, barY, fillWidth, barHeight, borderRadius);
            this.ctx.fill();
          }
          
          // Border
          this.ctx.strokeStyle = 'rgba(64, 64, 64, 0.8)';
          this.ctx.lineWidth = 2;
          this.drawRoundedRect(barX, barY, barWidth, barHeight, borderRadius);
          this.ctx.stroke();
          
          // Text inside the bar
          this.ctx.fillStyle = '#FFFFFF';
          this.ctx.font = 'bold 12px Arial';
          
          // Center: Stat name
          const centerText = `${statNames[statKey]}`;
          const centerTextWidth = this.ctx.measureText(centerText).width;
          this.ctx.fillText(centerText, barX + 10 + centerTextWidth / 2, barY + 14);
          
          // Right side: Cost or MAX (inside bar)
          let rightText;
          if (level < maxLevel) {
            rightText = `$${cost}`;
            this.ctx.fillStyle = '#FFFFFF';
          } else {
            rightText = 'MAX';
            this.ctx.fillStyle = '#FFFFFF';
          }
          const rightTextWidth = this.ctx.measureText(rightText).width;
          this.ctx.fillText(rightText, barX + barWidth - rightTextWidth - 5, barY + 14);
          
          // Key number to the right of the bar (white)
          this.ctx.fillStyle = '#FFFFFF';
          this.ctx.font = 'bold 14px Arial';
          this.ctx.fillText(keyNumber, barX + barWidth + 10, barY + 14);
          
          yOffset += 25; // Spacing between individual bars with padding
        }
      });
      
      // Instructions
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.font = '11px Arial';
      this.ctx.textAlign = 'left';
      this.ctx.fillText('Press 1-8 to upgrade stats', panelX, yOffset + 20);
    } else {
      this.ctx.fillStyle = '#B0B0B0';
      this.ctx.font = '12px Arial';
      this.ctx.textAlign = 'left';
      this.ctx.fillText('No stat upgrade data available', panelX, coinsBarY + coinsBarHeight + 30);
    }
  }

  drawLeaderboard() {
    const sortedPlayers = [...this.gameState.players]
      .filter(player => !player.isBot) // Exclude bots from leaderboard
      .sort((a, b) => {
        const scoreDiff = (b.score || 0) - (a.score || 0);
        if (scoreDiff !== 0) return scoreDiff;
        // If scores are tied, sort alphabetically by name
        const nameA = (a.name || `Player ${a.id}`).toLowerCase();
        const nameB = (b.name || `Player ${b.id}`).toLowerCase();
        return nameA.localeCompare(nameB);
      })
      .slice(0, 5); // Top 5 players
    
    if (sortedPlayers.length === 0) return;
    
    const leaderboardWidth = 180;
    const leaderboardHeight = 30 + sortedPlayers.length * 25;
    const x = this.screenWidth - leaderboardWidth - 10;
    const y = 10;
    
    // Background
    this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
    this.drawRoundedRect(x, y, leaderboardWidth, leaderboardHeight, 5);
    this.ctx.fill();
    
    // Title
    this.ctx.fillStyle = '#FFFFFF';
    this.ctx.font = 'bold 16px Arial';
    this.ctx.textAlign = 'left';
    this.ctx.fillText('Leaderboard', x + 10, y + 20);
    
    // Players
    this.ctx.font = '14px Arial';
    sortedPlayers.forEach((player, index) => {
      const isMe = player.id === this.myPlayerId;
      this.ctx.fillStyle = isMe ? '#FFFFFF' : '#B0B0B0';
      
      const rank = index + 1;
      const name = player.name || `Player ${player.id}`;
      const score = player.score || 0;
      
      this.ctx.fillText(`${rank}. ${name}`, x + 10, y + 45 + index * 25);
      this.ctx.textAlign = 'right';
      this.ctx.fillText(`${score}`, x + leaderboardWidth - 10, y + 45 + index * 25);
      this.ctx.textAlign = 'left';
    });
  }

  drawControls() {
    const controlsWidth = 220;
    const controlsHeight = 190;
    const x = 10;
    const y = this.screenHeight - controlsHeight - 10;
    
    // Background
    this.ctx.fillStyle = 'rgba(64, 64, 64, 0.7)';
    this.ctx.fillRect(x, y, controlsWidth, controlsHeight);
    
    // Controls text
    this.ctx.fillStyle = '#FFFFFF';
    this.ctx.font = '14px Arial';
    this.ctx.textAlign = 'left';
    
    this.ctx.fillText('CONTROLS:', x + 10, y + 20);
    this.ctx.font = '12px Arial';
    this.ctx.fillText('WASD: Move', x + 10, y + 40);
    this.ctx.fillText('Q: Fire Left Cannons', x + 10, y + 55);
    this.ctx.fillText('E: Fire Right Cannons', x + 10, y + 70);
    this.ctx.fillText('Mouse: Aim Turrets', x + 10, y + 85);
    this.ctx.fillText('L: Debug Level Up', x + 10, y + 100);
    this.ctx.fillText('Click: Select Upgrades', x + 10, y + 115);
    this.ctx.fillText('+/-: Add/Remove Cannons', x + 10, y + 130);
    this.ctx.fillText('P/O: Add/Remove Scatter', x + 10, y + 145);
    this.ctx.fillText('[/]: Add/Remove Turrets', x + 10, y + 160);
  }

  drawMinimap() {
    const minimapSize = 160;
    const minimapX = this.screenWidth - minimapSize - 20;
    const minimapY = this.screenHeight - minimapSize - 20;
    
    // Background
    this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
    this.ctx.fillRect(minimapX, minimapY, minimapSize, minimapSize);
    
    // Border
    this.ctx.strokeStyle = '#FFFFFF';
    this.ctx.lineWidth = 2;
    this.ctx.strokeRect(minimapX, minimapY, minimapSize, minimapSize);
    
    // Scale factors
    const scaleX = minimapSize / WorldWidth;
    const scaleY = minimapSize / WorldHeight;
    
    // Draw players as dots
    this.gameState.players.forEach(player => {
      const dotX = minimapX + (player.x * scaleX);
      const dotY = minimapY + (player.y * scaleY);
      const dotSize = Math.max(2, player.size * scaleX * 0.5);
      
      this.ctx.fillStyle = player.id === this.myPlayerId ? '#FFFFFF' : '#B0B0B0';
      this.ctx.beginPath();
      this.ctx.arc(dotX, dotY, dotSize, 0, Math.PI * 2);
      this.ctx.fill();
    });
    
    // Draw current player position as a small white dot (using predicted position for responsiveness)
    if (this.predictedPlayerPos) {
      const playerDotX = minimapX + (this.predictedPlayerPos.x * scaleX);
      const playerDotY = minimapY + (this.predictedPlayerPos.y * scaleY);
      
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.beginPath();
      this.ctx.arc(playerDotX, playerDotY, 2, 0, Math.PI * 2);
      this.ctx.fill();
    }
    
    // Draw items as smaller dots
    this.ctx.fillStyle = 'rgba(255, 215, 0, 0.6)';
    this.gameState.items.forEach(item => {
      const dotX = minimapX + (item.x * scaleX);
      const dotY = minimapY + (item.y * scaleY);
      
      this.ctx.beginPath();
      this.ctx.arc(dotX, dotY, 1, 0, Math.PI * 2);
      this.ctx.fill();
    });
  }

  drawLevelProgressBar() {
    if (!this.gameState.myPlayer) return;
    
    const player = this.gameState.myPlayer;
    const barWidth = 400;
    const barHeight = 30;
    const barX = (this.screenWidth - barWidth) / 2;
    const barY = this.screenHeight - 60;
    
    // Progress bar background
    this.ctx.fillStyle = '#404040'; // Dark gray background
    this.drawRoundedRect(barX, barY, barWidth, barHeight, 5);
    this.ctx.fill();
    
    // Progress bar fill
    const progress = this.getExperienceProgress(player);
    this.ctx.fillStyle = '#B0B0B0';
    this.drawRoundedRect(barX, barY, barWidth * progress, barHeight, 5);
    this.ctx.fill();
    
    // Progress bar border
    this.ctx.strokeStyle = '#404040'; // Dark gray border
    this.ctx.lineWidth = 2;
    this.drawRoundedRect(barX, barY, barWidth, barHeight, 5);
    this.ctx.stroke();
    
    // Text
    this.ctx.fillStyle = '#FFFFFF';
    this.ctx.font = 'bold 16px Arial';
    this.ctx.textAlign = 'center';
    
    // Experience text
    const currentLevelExp = this.getExperienceForLevel(player.level || 1);
    const nextLevelExp = this.getExperienceForLevel((player.level || 1) + 1);
    const currentExp = player.experience || 0;
    const progressPercent = Math.round(((currentExp - currentLevelExp) / (nextLevelExp - currentLevelExp)) * 100);
    this.ctx.font = '14px Arial';
    this.ctx.fillText(`${progressPercent}%`, this.screenWidth / 2, barY + 20);
    
    // Available upgrades indicator
    if (player.availableUpgrades > 0) {
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.font = 'bold 18px Arial';
      this.ctx.fillText(`${player.availableUpgrades} Modules${player.availableUpgrades > 1 ? 's' : ''} Available!`, this.screenWidth / 2, barY - 20);
    }
  }
  
  drawUpgradeUI() {
    if (!this.gameState.myPlayer) return;
    
    const player = this.gameState.myPlayer;
    if (player.availableUpgrades <= 0) {
      return;
    }
    
    // Collect available upgrade types
    const availableTypes = [];
    const upgradeTypes = ['side', 'top', 'front', 'rear'];
    
    for (const type of upgradeTypes) {
      if (this.hasAvailableUpgrades(type)) {
        availableTypes.push(type);
      }
    }
    
    if (availableTypes.length === 0) return;
    
    // Draw upgrade type buttons (always centered based on available types)
    const buttonWidth = 50;
    const buttonHeight = 50;
    const spacing = 20;
    const totalWidth = (buttonWidth * availableTypes.length) + (spacing * (availableTypes.length - 1));
    const startX = (this.screenWidth - totalWidth) / 2;
    const buttonY = this.screenHeight - 150;
    
    const buttonPositions = {};
    
    for (let i = 0; i < availableTypes.length; i++) {
      const type = availableTypes[i];
      const x = startX + (buttonWidth + spacing) * i;
      buttonPositions[type] = x;
      
      // Button background
      this.ctx.fillStyle = this.upgradeUI.selectedUpgradeType === type ? '#B0B0B0' : 'rgba(64, 64, 64, 0.8)';
      this.drawRoundedRect(x, buttonY, buttonWidth, buttonHeight, 5);
      this.ctx.fill();
      
      // Button border
      this.ctx.strokeStyle = 'rgba(64, 64, 64, 0.8)';
      this.ctx.lineWidth = 2;
      this.drawRoundedRect(x, buttonY, buttonWidth, buttonHeight, 5);
      this.ctx.stroke();
      
      // Button text
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.font = 'bold 14px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText(type.toUpperCase(), x + buttonWidth / 2, buttonY + buttonHeight / 2 + 6);
      
      // Draw upgrade options only for the selected type
      if (this.upgradeUI.selectedUpgradeType === type) {
        this.drawUpgradeOptionsForType(type, x, buttonY, buttonWidth);
      }
    }
  }
  
  drawUpgradeOptionsForType(upgradeType, buttonX, buttonY, buttonWidth) {
    const options = this.getAvailableUpgrades(upgradeType);
    if (!options || options.length === 0) return;
    
    const optionHeight = 30;
    const optionWidth = Math.max(buttonWidth, 125); // At least as wide as button
    const spacing = 10;
    const totalHeight = (optionHeight * options.length) + (spacing * (options.length - 1));
    
    // Position options directly above the button, centered on it
    const optionsX = buttonX + (buttonWidth - optionWidth) / 2;
    const optionsStartY = buttonY - totalHeight - 10; // 20px gap above button
    
    // Clear all option positions and only store for the selected type
    this.upgradeUI.optionPositions = {};
    this.upgradeUI.optionPositions[upgradeType] = [];
    
    for (let i = 0; i < options.length; i++) {
      const option = options[i];
      const y = optionsStartY + (optionHeight + spacing) * i;
      
      // Store position for click detection
      this.upgradeUI.optionPositions[upgradeType].push({
        x: optionsX,
        y: y,
        width: optionWidth,
        height: optionHeight,
        option: option
      });
      
      // Option background
      this.ctx.fillStyle = 'rgba(64, 64, 64, 0.8)';
      this.drawRoundedRect(optionsX, y, optionWidth, optionHeight, 5);
      this.ctx.fill();
      
      // Option border
      this.ctx.strokeStyle = 'rgba(64, 64, 64, 0.8)';
      this.ctx.lineWidth = 2;
      this.drawRoundedRect(optionsX, y, optionWidth, optionHeight, 5);
      this.ctx.stroke();
      
      // Option text
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.font = '14px Arial';
      this.ctx.textAlign = 'center';
      this.ctx.fillText(option.name, optionsX + optionWidth / 2, y + optionHeight / 2 + 5);
    }
  }
  
  // Helper functions
  getExperienceForLevel(level) {
    // Progressive increment: each level requires 100 more XP than the previous level's increment
    // Level 1 = 0, Level 2 = 100, Level 3 = 300, Level 4 = 600, Level 5 = 1000, etc.
    if (level <= 1) {
      return 0;
    }
    
    let totalExp = 0;
    
    for (let i = 2; i <= level; i++) {
      // Level increment increases by 100 each level: 100, 200, 300, 400...
      const levelIncrement = (i - 1) * 100;
      totalExp += levelIncrement;
    }
    
    return totalExp;
  }
  
  getExperienceProgress(player) {
    const currentLevel = player.level || 1;
    const currentLevelExp = this.getExperienceForLevel(currentLevel);
    const nextLevelExp = this.getExperienceForLevel(currentLevel + 1);
    const currentExp = player.experience || 0;
    
    const progress = (currentExp - currentLevelExp) / (nextLevelExp - currentLevelExp);
    return Math.max(0, Math.min(1, progress));
  }
  
  hasAvailableUpgrades(upgradeType) {
    const upgrades = this.upgradeUI.availableUpgrades[upgradeType];
    return upgrades && upgrades.length > 0;
  }
  
  getAvailableUpgrades(upgradeType) {
    return this.upgradeUI.availableUpgrades[upgradeType] || [];
  }

  updateConnectionStatus(connected) {
    const statusElement = document.querySelector('#connectionStatus');
    if (!statusElement) {
      return;
    }
    const indicator = statusElement.querySelector('.status-indicator');
    const text = statusElement.querySelector('span:last-child');
    if (!indicator || !text) {
      return;
    }
    
    if (connected) {
      indicator.className = 'status-indicator connected';
      text.textContent = 'Connected';
    } else {
      indicator.className = 'status-indicator disconnected';
      text.textContent = 'Connecting...';
    }
  }

  startGameLoop() {
    let lastFrameTime = 0;
    const targetFPS = 60;
    const frameInterval = 1000 / targetFPS;
    
    const gameLoop = (currentTime) => {
      if (currentTime - lastFrameTime >= frameInterval) {
        this.render();
        lastFrameTime = currentTime;
      }
      requestAnimationFrame(gameLoop);
    };
    gameLoop();
  }

  applyProfile(playerName, playerColor) {
    let updated = false;

    const sanitizedName = sanitizePlayerName(playerName);
    if (sanitizedName && sanitizedName !== this.playerConfig.name) {
      this.playerConfig.name = sanitizedName;
      if (this.gameState.myPlayer) {
        this.gameState.myPlayer.name = sanitizedName;
      }
      updated = true;
    }

    const sanitizedColor = sanitizeHexColor(playerColor);
    if (sanitizedColor && sanitizedColor !== this.playerConfig.color) {
      this.playerConfig.color = sanitizedColor;
      if (this.gameState.myPlayer) {
        this.gameState.myPlayer.color = sanitizedColor;
      }
      updated = true;
    }

    if (updated || !this.socket) {
      this.pendingConnectConfig = { ...this.playerConfig };
      this.connect(
        { playerName: this.playerConfig.name, playerColor: this.playerConfig.color },
        { force: true }
      );
    }
  }

  setControlsLocked(locked) {
    this.controlsLocked = Boolean(locked);
    if (this.controlsLocked) {
      this.clearActiveInputs();
      // send one last update that all inputs are cleared
      if (this.socket && this.socket.readyState === WebSocket.OPEN) {
        this.socket.send(JSON.stringify(this.input));
      }
    } else {
      this.sendInput();
    }
  }

  sendStartGame() {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({
        type: 'startGame',
        startGame: true
      }));
      this.hasStartedGame = true; // Mark that player has started the game
      console.log('Sent startGame message to server');
    } else {
      console.log('Cannot send startGame: socket not ready');
    }
  }

  clearActiveInputs() {
    this.input.up = false;
    this.input.down = false;
    this.input.left = false;
    this.input.right = false;
    this.input.shootLeft = false;
    this.input.shootRight = false;
    this.input.upgradeCannons = false;
    this.input.downgradeCannons = false;
    this.input.upgradeScatter = false;
    this.input.downgradeScatter = false;
    this.input.upgradeTurrets = false;
    this.input.downgradeTurrets = false;
    this.input.debugLevelUp = false;
    this.input.selectUpgrade = '';
    this.input.upgradeChoice = '';
    this.input.statUpgradeType = '';
    this.input.toggleAutofire = false;
  }

  drawAutofireStatus() {
    if (!this.gameState.myPlayer) return;
    
    const player = this.gameState.myPlayer;
    
    // Use client-side predicted state if available and timeout hasn't expired, otherwise use server state
    const serverState = player.autofireEnabled !== false;
    const autofireEnabled = this.clientAutofireState !== null ? this.clientAutofireState : serverState;
    
    // Position at bottom left of screen
    const padding = 20;
    const textWidth = 140;
    const textHeight = 24;
    const boxX = padding;
    const boxY = this.screenHeight - padding - textHeight;
    const textX = padding + textWidth/2;
    const textY = this.screenHeight - padding - textHeight/2;
    
    this.ctx.save();
    
    // Draw background box - using game colors
    this.ctx.fillStyle = autofireEnabled ? 'rgba(76, 175, 80, 0.7)' : 'rgba(255, 68, 68, 0.7)';
    this.drawRoundedRect(boxX, boxY, textWidth, textHeight, 8);
    this.ctx.fill();
    
    // Draw border - using game colors
    this.ctx.strokeStyle = autofireEnabled ? '#4CAF50' : '#ff4444';
    this.ctx.lineWidth = 3;
    this.drawRoundedRect(boxX, boxY, textWidth, textHeight, 8);
    this.ctx.stroke();
    
    // Draw text
    this.ctx.fillStyle = '#ffffff';
    this.ctx.font = 'bold 14px Arial';
    this.ctx.textAlign = 'center';
    this.ctx.textBaseline = 'middle';
    this.ctx.fillText(`Autofire: ${autofireEnabled ? 'ON' : 'OFF'} (R)`, textX, textY);
    
    this.ctx.restore();
  }

  // Merges a delta player update into an existing full player object
  mergeDeltaPlayer(existingPlayer, deltaPlayer) {
    const merged = { ...existingPlayer };

    // Apply only the changed fields from the delta
    if (deltaPlayer.x !== undefined) merged.x = deltaPlayer.x;
    if (deltaPlayer.y !== undefined) merged.y = deltaPlayer.y;
    if (deltaPlayer.velX !== undefined) merged.velX = deltaPlayer.velX;
    if (deltaPlayer.velY !== undefined) merged.velY = deltaPlayer.velY;
    if (deltaPlayer.angle !== undefined) merged.angle = deltaPlayer.angle;
    if (deltaPlayer.score !== undefined) merged.score = deltaPlayer.score;
    if (deltaPlayer.state !== undefined) merged.state = deltaPlayer.state;
    if (deltaPlayer.name !== undefined) merged.name = deltaPlayer.name;
    if (deltaPlayer.color !== undefined) merged.color = deltaPlayer.color;
    if (deltaPlayer.health !== undefined) merged.health = deltaPlayer.health;
    if (deltaPlayer.maxHealth !== undefined) merged.maxHealth = deltaPlayer.maxHealth;
    if (deltaPlayer.level !== undefined) merged.level = deltaPlayer.level;
    if (deltaPlayer.experience !== undefined) merged.experience = deltaPlayer.experience;
    if (deltaPlayer.availableUpgrades !== undefined) merged.availableUpgrades = deltaPlayer.availableUpgrades;
    if (deltaPlayer.shipConfig !== undefined) merged.shipConfig = deltaPlayer.shipConfig; // Always present now
    if (deltaPlayer.coins !== undefined) merged.coins = deltaPlayer.coins;
    if (deltaPlayer.statUpgrades !== undefined) merged.statUpgrades = deltaPlayer.statUpgrades;
    if (deltaPlayer.autofireEnabled !== undefined) merged.autofireEnabled = deltaPlayer.autofireEnabled;
    if (deltaPlayer.debugInfo !== undefined) merged.debugInfo = deltaPlayer.debugInfo;

    return merged;
  }

  // Converts a delta player to a full player object (for new players)
  deltaToFullPlayer(deltaPlayer) {
    return {
      id: deltaPlayer.id,
      x: deltaPlayer.x || 0,
      y: deltaPlayer.y || 0,
      velX: deltaPlayer.velX || 0,
      velY: deltaPlayer.velY || 0,
      angle: deltaPlayer.angle || 0,
      score: deltaPlayer.score || 0,
      state: deltaPlayer.state || 0,
      name: deltaPlayer.name || `Player ${deltaPlayer.id}`,
      color: deltaPlayer.color || '#FF6B6B',
      isBot: false, // Delta players are never bots
      health: deltaPlayer.health || 100,
      maxHealth: deltaPlayer.maxHealth || 100,
      level: deltaPlayer.level || 1,
      experience: deltaPlayer.experience || 0,
      availableUpgrades: deltaPlayer.availableUpgrades || 0,
      shipConfig: deltaPlayer.shipConfig || {}, // Always present now
      coins: deltaPlayer.coins || 0,
      statUpgrades: deltaPlayer.statUpgrades || {},
      autofireEnabled: deltaPlayer.autofireEnabled || false,
      debugInfo: deltaPlayer.debugInfo || {}
    };
  }
}

function sanitizePlayerName(name) {
  if (!name || typeof name !== 'string') {
    return '';
  }
  const cleaned = name.trim().replace(/[^a-zA-Z0-9\s'-]/g, '');
  return cleaned.slice(0, 16).trim();
}

function sanitizeHexColor(color) {
  if (!color || typeof color !== 'string') {
    return '';
  }
  const match = /^#?([0-9a-fA-F]{6})$/.exec(color.trim());
  return match ? `#${match[1].toUpperCase()}` : '';
}

function generateRandomName() {
  const base = NAME_POOL[Math.floor(Math.random() * NAME_POOL.length)];
  const suffix = Math.floor(100 + Math.random() * 900);
  return `${base} ${suffix}`;
}

class StartScreen {
  constructor(clientInstance) {
    this.client = clientInstance;
    this.element = document.getElementById('startScreen');
    if (!this.element) {
      if (this.client) {
        this.client.setControlsLocked(false);
      }
      return;
    }

    document.body.classList.remove('has-launched');

    this.form = document.getElementById('startForm');
    this.nameInput = document.getElementById('playerName');
    this.colorInput = document.getElementById('playerColor');
    this.colorPreview = document.getElementById('colorPreviewValue');
    this.swatchContainer = document.getElementById('presetColors');
    this.randomButton = document.getElementById('randomizeName');
    this.playButton = this.form ? this.form.querySelector('.play-button') : null;
    this.activeSwatch = null;
    this.hasLaunched = false;

    if (this.client) {
      this.client.setControlsLocked(true);
    }

    this.populateSwatches();
    const initialColor = (this.client && this.client.playerConfig.color) || (this.colorInput ? this.colorInput.value : PRESET_COLORS[2]);
    const initialName = (this.client && this.client.playerConfig.name) || (this.nameInput ? this.nameInput.value : '');

    const sanitizedColor = this.applyColor(initialColor);
    const sanitizedName = this.applyName(initialName);
    if (this.client) {
      this.client.applyProfile(sanitizedName, sanitizedColor);
    }
    this.registerEvents();
  }

  populateSwatches() {
    if (!this.swatchContainer) {
      return;
    }

    this.swatchContainer.innerHTML = '';
    PRESET_COLORS.forEach((hex) => {
      const sanitized = sanitizeHexColor(hex);
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'color-swatch';
      button.dataset.color = sanitized;
      button.style.setProperty('--swatch-color', sanitized);
      button.setAttribute('aria-label', `Use ${sanitized} hull`);
      button.addEventListener('click', () => {
        this.applyColor(sanitized);
      });
      this.swatchContainer.appendChild(button);
    });
  }

  registerEvents() {
    if (this.form) {
      this.form.addEventListener('submit', (event) => {
        event.preventDefault();
        this.launchGame();
      });
    }

    if (this.colorInput) {
      this.colorInput.addEventListener('input', (event) => {
        this.applyColor(event.target.value);
      });
    }

    if (this.nameInput) {
      this.nameInput.addEventListener('blur', () => {
        this.applyName(this.nameInput.value);
      });
    }

    if (this.randomButton) {
      this.randomButton.addEventListener('click', () => {
        const randomName = generateRandomName();
        this.applyName(randomName);
        if (this.nameInput) {
          this.nameInput.focus();
          this.nameInput.select();
        }
      });
    }
  }

  applyName(value) {
    const sanitized = sanitizePlayerName(value) || generateRandomName();
    if (this.nameInput) {
      this.nameInput.value = sanitized;
    }
    return sanitized;
  }

  applyColor(value) {
    const sanitized = sanitizeHexColor(value) || sanitizeHexColor(PRESET_COLORS[2]);
    if (this.colorInput) {
      this.colorInput.value = sanitized;
    }
    if (this.colorPreview) {
      this.colorPreview.textContent = sanitized;
    }
    if (this.swatchContainer) {
      this.swatchContainer.querySelectorAll('.color-swatch').forEach((button) => {
        if (button.dataset.color === sanitized) {
          button.classList.add('active');
          this.activeSwatch = button;
        } else {
          button.classList.remove('active');
        }
      });
    }
    return sanitized;
  }

  launchGame() {
    if (this.hasLaunched) {
      return;
    }

    const chosenName = this.applyName(this.nameInput ? this.nameInput.value : '');
    const chosenColor = this.applyColor(this.colorInput ? this.colorInput.value : PRESET_COLORS[2]);

    if (this.playButton) {
      this.playButton.disabled = true;
      this.playButton.textContent = 'Launching...';
    }

    const statusIndicator = document.querySelector('#connectionStatus .status-indicator');
    const statusText = document.querySelector('#connectionStatus span:last-child');
    if (statusIndicator && statusText) {
      statusIndicator.className = 'status-indicator disconnected';
      statusText.textContent = 'Connecting...';
    }

    this.hasLaunched = true;
    this.element.classList.add('hidden');
    document.body.classList.add('has-launched');

    if (this.client) {
      // Update profile locally
      this.client.playerConfig.name = chosenName;
      this.client.playerConfig.color = chosenColor;
      
      // Send profile update to server
      if (this.client.socket && this.client.socket.readyState === WebSocket.OPEN) {
        this.client.socket.send(JSON.stringify({
          type: 'profile',
          playerName: chosenName,
          playerColor: chosenColor
        }));
      }
      
      // Ensure client is connected before sending start game
      if (!this.client.socket || this.client.socket.readyState !== WebSocket.OPEN) {
        this.client.connect({ playerName: chosenName, playerColor: chosenColor });
        // Set flag to send startGame after connection
        this.client.shouldStartGame = true;
      } else {
        // Client is already connected, send startGame immediately
        this.client.sendStartGame();
      }
      
      this.client.setControlsLocked(false);
    } else {
      window.goblonsClient = new GameClient({
        playerName: chosenName,
        playerColor: chosenColor,
        shouldStartGame: true // Flag to send startGame after connecting
      });
    }
  }
}

window.addEventListener('load', () => {
  const client = new GameClient({ autoConnect: false });
  window.goblonsClient = client;
  window.goblonsIntro = new StartScreen(client);
});
