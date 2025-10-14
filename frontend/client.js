// Game constants (should match backend)
const WorldWidth = 2000.0;
const WorldHeight = 2000.0;

class GameClient {
  constructor() {
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
      up: false, 
      down: false, 
      left: false, 
      right: false,
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
      mouse: { x: 0, y: 0 }
    };
    
    // Ship physics properties for client-side prediction

    this.shipPhysics = {
      angle: 0,           // Current facing direction (radians)
      velocity: { x: 0, y: 0 },  // Current velocity
      acceleration: 1000,   // Forward acceleration (matches server)
      deceleration: 0.97,  // Drag/friction factor (matches server)
      turnSpeed: 0.07,      // How fast the ship turns (matches server)
      maxSpeed: 4.0        // Maximum speed (matches server)
    };
    
    this.camera = { x: 0, y: 0, targetX: 0, targetY: 0 };
    this.isConnected = false;
    this.screenWidth = window.innerWidth;
    this.screenHeight = window.innerHeight;
    
        // UI state for upgrade system
    this.upgradeUI = {
      selectedUpgradeType: null, // 'side', 'top', 'front', 'rear'
      availableUpgrades: {},     // stores available upgrades for each type
      pendingUpgrade: false,     // prevents multiple upgrade selections
      optionPositions: {},       // stores click positions for upgrade options
    };
    
    // Track last mouse screen position for camera movement updates
    this.lastMouseScreen = { x: 0, y: 0 };
    this.lastCameraPos = { x: 0, y: 0 };
    
    // Client-side prediction
    this.predictedPlayerPos = { x: 0, y: 0 };
    this.lastPredictionUpdate = Date.now();
    
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
    this.connect();
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
    });
    
    document.addEventListener('keyup', (e) => {
      this.handleKeyUp(e);
    });
    
    // Mouse input
    this.canvas.addEventListener('mousemove', (e) => {
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
    
    // Mouse click handling for upgrade UI
    this.canvas.addEventListener('click', (e) => {
      const rect = this.canvas.getBoundingClientRect();
      const screenX = e.clientX - rect.left;
      const screenY = e.clientY - rect.top;
      
      this.handleUpgradeUIClick(screenX, screenY);
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

  connect() {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
    this.socket = new WebSocket(`${protocol}//${location.host}/ws`);
    
    this.socket.onopen = () => {
      console.log('Connected to server');
      this.isConnected = true;
      this.updateConnectionStatus(true);
    };
    
    this.socket.onmessage = (event) => {
      const data = JSON.parse(event.data);
      this.handleMessage(data);
    };
    
    this.socket.onclose = () => {
      console.log('Disconnected from server');
      this.isConnected = false;
      this.updateConnectionStatus(false);
      // Try to reconnect after 3 seconds
      setTimeout(() => this.connect(), 3000);
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
              this.gameState.myPlayer = serverPlayer;
              
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
      case 'itemCollected':
        // Could add visual effects for item collection
        break;
    }
  }

  handleKeyDown(e) {
    let inputChanged = false;
    
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
    
    // Handle stat upgrade keys (1-8)
    if (e.key >= '1' && e.key <= '8') {
      this.handleStatUpgradeKey(parseInt(e.key));
      // Don't set inputChanged since we handle this separately
    }
    
    if (inputChanged) {
      this.sendInput();
    }
  }

  handleKeyUp(e) {
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
    if (!this.gameState.myPlayer || this.gameState.myPlayer.availableUpgrades <= 0 || this.upgradeUI.pendingUpgrade) return;
    // First check if clicking on upgrade type buttons
    const availableTypes = [];
    const upgradeTypes = ['side', 'top', 'front', 'rear'];
    for (const type of upgradeTypes) {
      if (this.hasAvailableUpgrades(type)) {
        availableTypes.push(type);
      }
    }
    if (availableTypes.length > 0) {
      // Match button dimensions to drawUpgradeUI
      const buttonWidth = 50;
      const buttonHeight = 50;
      const spacing = 20;
      const totalWidth = (buttonWidth * availableTypes.length) + (spacing * (availableTypes.length - 1));
      const startX = (this.screenWidth - totalWidth) / 2;
      const buttonY = this.screenHeight - 150;
      for (let i = 0; i < availableTypes.length; i++) {
        const type = availableTypes[i];
        const x = startX + (buttonWidth + spacing) * i;
        if (screenX >= x && screenX <= x + buttonWidth && 
            screenY >= buttonY && screenY <= buttonY + buttonHeight) {
          // Toggle selection - if already selected, deselect; otherwise select
          if (this.upgradeUI.selectedUpgradeType === type) {
            this.upgradeUI.selectedUpgradeType = null;
          } else {
            this.upgradeUI.selectedUpgradeType = type;
          }
          return;
        }
      }
    }
    
    // Then check upgrade option clicks using stored positions
    if (this.upgradeUI.optionPositions && this.upgradeUI.selectedUpgradeType) {
      const positions = this.upgradeUI.optionPositions[this.upgradeUI.selectedUpgradeType];
      if (positions) {
        for (const pos of positions) {
          if (screenX >= pos.x && screenX <= pos.x + pos.width && 
              screenY >= pos.y && screenY <= pos.y + pos.height) {
            // Select upgrade
            this.selectUpgrade(this.upgradeUI.selectedUpgradeType, pos.option.name);
            return;
          }
        }
      }
    }
  }
  
  selectUpgrade(upgradeType, upgradeId) {
    // Prevent multiple upgrade selections
    if (this.upgradeUI.pendingUpgrade) {
      return;
    }
    
    // Mark as pending to prevent duplicate selections
    this.upgradeUI.pendingUpgrade = true;
    
    // Send upgrade selection to server
    this.input.selectUpgrade = upgradeType;
    this.input.upgradeChoice = upgradeId;
    this.sendInput();
    
    // Reset upgrade selection inputs immediately to prevent repeated sending
    this.input.selectUpgrade = '';
    this.input.upgradeChoice = '';
    
    // Clear selected upgrade type to hide the options
    this.upgradeUI.selectedUpgradeType = null;
    
    // Reset pending flag after a delay (or when server confirms upgrade)
    setTimeout(() => {
      this.upgradeUI.pendingUpgrade = false;
    }, 1000);
  }

  handleStatUpgradeKey(keyNumber) {
    if (!this.gameState.myPlayer || !this.gameState.myPlayer.statUpgrades) return;
    
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
    if (!statKey) return;
    
    const statUpgrade = player.statUpgrades[statKey];
    if (!statUpgrade) return;
    
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
      return;
    }
    
    if (totalUpgrades >= 75) {
      console.log(`Cannot upgrade ${statNames[statKey]} - Total upgrade limit reached (75/75)`);
      return;
    }
    
    if (coins < cost) {
      console.log(`Not enough coins to upgrade ${statNames[statKey]}. Need ${cost}, have ${coins}`);
      return;
    }
    
    // Send stat upgrade request
    this.input.statUpgradeType = statKey;
    this.sendInput();
    this.input.statUpgradeType = ''; // Clear immediately
    console.log(`Upgrading ${statNames[statKey]} (Level ${level} -> ${level + 1}) for ${cost} coins`);
  }


  sendInput() {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(this.input));
    }
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
    
    // Update predicted position with more conservative movement
    const moveX = physics.velocity.x * deltaTime * 30; // Reduced multiplier
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
  }

  drawGrid() {
    const gridSize = 25;
    this.ctx.strokeStyle = '#808080';
    this.ctx.lineWidth = 1;
    
    const startX = Math.floor(this.camera.x / gridSize) * gridSize;
    const startY = Math.floor(this.camera.y / gridSize) * gridSize;
    
    for (let x = startX; x < this.camera.x + this.screenWidth; x += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(x - this.camera.x, 0);
      this.ctx.lineTo(x - this.camera.x, this.screenHeight);
      this.ctx.stroke();
    }
    
    for (let y = startY; y < this.camera.y + this.screenHeight; y += gridSize) {
      this.ctx.beginPath();
      this.ctx.moveTo(0, y - this.camera.y);
      this.ctx.lineTo(this.screenWidth, y - this.camera.y);
      this.ctx.stroke();
    }
  }

  drawMapBorder() {
    const worldWidth = 2000;
    const worldHeight = 2000;
    
    // Convert world coordinates to screen coordinates
    const borderLeft = 0 - this.camera.x;
    const borderTop = 0 - this.camera.y;
    const borderRight = worldWidth - this.camera.x;
    const borderBottom = worldHeight - this.camera.y;
    
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
  if (player.shipConfig && player.shipConfig.topUpgrade && player.shipConfig.topUpgrade.turrets && player.shipConfig.topUpgrade.turrets.length == 0) {
    ctx.beginPath();
    ctx.arc(0, 0, shaftWidth * 0.2, 0, Math.PI * 2);
    ctx.fillStyle = '#444';
    ctx.fill();
    ctx.strokeStyle = '#444';
    ctx.stroke();
  }

  // --- Draw cannons using new modular system ---
  ctx.fillStyle = '#444';

  // Draw side cannons from modular system
  if (player.shipConfig && player.shipConfig.sideUpgrade && player.shipConfig.sideUpgrade.cannons) {
    for (const cannon of player.shipConfig.sideUpgrade.cannons) {
      // Backend provides relative positions, draw cannon centered on that position
      const centerX = cannon.position.x;
      const centerY = cannon.position.y;
      
      if (cannon.type === 'scatter') {
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
        ctx.lineWidth = 1;
        ctx.stroke();
      } else {
        // Draw regular cannon as rectangle
        const x = centerX - gunLength / 2; // Convert center to top-left for fillRect
        const y = centerY - gunWidth / 2;  // Convert center to top-left for fillRect
        ctx.fillRect(x, y, gunLength, gunWidth);
      }
    }
  }

  // --- Draw turrets using new modular system ---
  if (player.shipConfig && player.shipConfig.topUpgrade && player.shipConfig.topUpgrade.turrets) {
    for (const turret of player.shipConfig.topUpgrade.turrets) {
      // Draw turret base (circular mount)
      const turretSize = size * 0.5;
      const barrelLength = size * 0.5;
      const barrelWidth = size * 0.2;
      
      ctx.save();
      ctx.translate(turret.position.x, turret.position.y);

      // Draw turret barrel(s) based on turret type
      ctx.rotate(turret.angle - angle);
      ctx.fillStyle = '#444';
      
      if (turret.type === 'twin_turret' && turret.cannons && turret.cannons.length >= 2) {
        // Draw two parallel barrels for twin turret
        const barrelSeparation = barrelWidth * 1.5;
        
        // Left barrel
        ctx.fillRect(0, -barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        ctx.strokeRect(0, -barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        
        // Right barrel
        ctx.fillRect(0, barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
        ctx.strokeRect(0, barrelSeparation/2 - barrelWidth/2, barrelLength, barrelWidth);
      } else {
        // Single barrel for regular turret
        ctx.fillRect(0, -barrelWidth / 2, barrelLength, barrelWidth);
        ctx.strokeRect(0, -barrelWidth / 2, barrelLength, barrelWidth);
      }
      
      // Draw turret base (slightly larger for twin turrets)
      const baseSize = turret.type === 'twin_turret' ? turretSize * 0.7 : turretSize * 0.5;
      ctx.fillStyle = '#555';
      ctx.beginPath();
      ctx.arc(0, 0, baseSize, 0, Math.PI * 2);
      ctx.fill();
      ctx.stroke();
      
      ctx.restore();
    }
  }

  ctx.restore();
  
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
      .sort((a, b) => (b.score || 0) - (a.score || 0))
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
    this.ctx.font = '14px Arial';
    this.ctx.fillText(`${currentExp - currentLevelExp}/${nextLevelExp - currentLevelExp} XP`, this.screenWidth / 2, barY + 20);
    
    // Available upgrades indicator
    if (player.availableUpgrades > 0) {
      this.ctx.fillStyle = '#FFFFFF';
      this.ctx.font = 'bold 18px Arial';
      this.ctx.fillText(`${player.availableUpgrades} Upgrade${player.availableUpgrades > 1 ? 's' : ''} Available!`, this.screenWidth / 2, barY - 20);
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
    // Exponential progression: each level requires 50% more experience than the previous
    // Level 1 = 0, Level 2 = 100, Level 3 = 250, Level 4 = 475, etc.
    if (level <= 1) {
      return 0;
    }
    
    let totalExp = 0;
    const baseExp = 100; // Experience needed to go from level 1 to 2
    
    for (let i = 2; i <= level; i++) {
      if (i === 2) {
        totalExp += baseExp;
      } else {
        // Each level requires 50% more than the previous level's requirement
        const levelExp = Math.floor(baseExp * Math.pow(2, i - 2));
        totalExp += levelExp;
      }
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
    const indicator = statusElement.querySelector('.status-indicator');
    const text = statusElement.querySelector('span:last-child');
    
    if (connected) {
      indicator.className = 'status-indicator connected';
      text.textContent = 'Connected';
    } else {
      indicator.className = 'status-indicator disconnected';
      text.textContent = 'Reconnecting...';
    }
  }

  startGameLoop() {
    const gameLoop = () => {
      this.render();
      requestAnimationFrame(gameLoop);
    };
    gameLoop();
  }
}

// Start the game when the page loads
window.addEventListener('load', () => {
  new GameClient();
});

// Game client is now initialized in the GameClient class above
