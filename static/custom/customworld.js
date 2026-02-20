const canvas = document.getElementById('world');
const ctx = canvas.getContext('2d');
canvas.width = window.innerWidth;
canvas.height = window.innerHeight;
const GRID_SIZE = 100;
let CELL_SIZE = Math.floor(Math.min(canvas.width, canvas.height) / GRID_SIZE);
let offsetX = Math.floor((canvas.width - CELL_SIZE * GRID_SIZE) / 2);
let offsetY = Math.floor((canvas.height - CELL_SIZE * GRID_SIZE) / 2);
let world = new Array(GRID_SIZE * GRID_SIZE).fill(0); // Initialize empty for custom builder
let previousWorld = [];
let stats = { left: 0, right: 0, speed: 1.0, paused: false };
let isDrawing = false;
let lastPlaceTime = 0;
const PLACE_DELAY = 100;
const speeds = [0.25, 0.5, 1, 2, 5, 10];
const minSpeed = speeds[0];
const maxSpeed = speeds[speeds.length - 1];
let currentTool = 3;
let brushSize = 1;
let fillMode = false;
const MAX_FILL_SIZE = 1000;
let lastClickX = 0;
let lastClickY = 0;
let mapBuilderPhase = 0; // 0=terrain, 1=borders, 2=features, 3=tribes, 4=done
let customMapData = {
    terrain: [],
    tribeAssignments: {}
};
let activeTribeConfigs = {}; // Maps tribeID -> {name, entityVizCode, homeTerrain}
let entityCodeToBiome = {}; // Maps entityVizCode -> biome name

// Store tribe names (received from backend)
let tribeNames = {
    "1": "Tribe 1",
    "2": "Tribe 2",
    "3": "Tribe 3",
    "4": "Tribe 4"
};

// ===== ZOOM AND PAN =====
let zoom = 1.0;
const MIN_ZOOM = 0.5;
const MAX_ZOOM = 4.0;
const ZOOM_SPEED = 0.1;
let isPanning = false;
let panStartX = 0;
let panStartY = 0;
let panOffsetX = 0;
let panOffsetY = 0;

// ===== BIOME SYSTEM - NORTH/SOUTH =====
const BIOMES = {
    GRASS: 'grass',
    SNOW: 'snow',        // North - Norsca (Vikings)
    DESERT: 'desert',
    CEMETERY: 'cemetery' // South - Sylvania (Vampires)
};

function getBiomeFromTerrain(terrainType) {
    // Terrain type 1 = red (north/snow), 2 = blue (south/cemetery)
    if (terrainType === 1) return BIOMES.GRASS;
    if (terrainType === 2) return BIOMES.SNOW;
    if (terrainType === 9) return BIOMES.DESERT;
    if (terrainType === 10) return BIOMES.CEMETERY;
    return null;
}

function getBiomeAt(x, y) {
    const idx = y * GRID_SIZE + x;
    const cell = world[idx];
    
    if (cell === 1 || cell === 2 || cell === 9 || cell === 10) {
        return getBiomeFromTerrain(cell);
    }
    
    // For features, check surrounding terrain
    let grassCount = 0;
    let snowCount = 0;
    let desertCount = 0;
    let cemeteryCount = 0;
    
    for (let dy = -1; dy <= 1; dy++) {
        for (let dx = -1; dx <= 1; dx++) {
            const nx = x + dx;
            const ny = y + dy;
            if (nx >= 0 && nx < GRID_SIZE && ny >= 0 && ny < GRID_SIZE) {
                const neighborIdx = ny * GRID_SIZE + nx;
                const neighborCell = world[neighborIdx];
                if (neighborCell === 1) grassCount++;
                if (neighborCell === 2) snowCount++;
                if (neighborCell === 9) desertCount++;
                if (neighborCell === 10) cemeteryCount++;
            }
        }
    }
    
    // Return biome with highest count
    const max = Math.max(grassCount, snowCount, desertCount, cemeteryCount);
    if (grassCount === max) return BIOMES.GRASS;
    if (snowCount === max) return BIOMES.SNOW;
    if (desertCount === max) return BIOMES.DESERT;
    if (cemeteryCount === max) return BIOMES.CEMETERY;
    
    // Fallback based on quadrant position
    if (x < 50 && y < 50) return BIOMES.GRASS;
    if (x > 50 && y < 50) return BIOMES.SNOW;
    if (x < 50 && y > 50) return BIOMES.DESERT;
    if (x > 50 && y > 50) return BIOMES.CEMETERY;
    return null;
}

function updateStatsDisplay(tribeData) {
    const statsDisplay = document.getElementById('statsDisplay');
    
    if (!tribeData || Object.keys(tribeData).length === 0) {
        statsDisplay.style.display = 'none';
        return;
    }
    
    // Clear existing content
    statsDisplay.innerHTML = '';
    
    // Create display for each active tribe
    Object.entries(tribeData).forEach(([tribeId, tribeInfo]) => {
        const tribeDiv = document.createElement('div');
        tribeDiv.style.marginBottom = '10px';
        
        tribeDiv.innerHTML = `
            <div>
                <span id="tribe${tribeId}Name">${tribeInfo.name || 'Tribe ' + tribeId}</span>: 
                <span id="tribe${tribeId}Count">0</span>
            </div>
            <div style="margin-left: 15px; font-size: 14px;">
                🪵 <span id="tribe${tribeId}Wood">0</span> | 
                🪨 <span id="tribe${tribeId}Stone">0</span>
            </div>
        `;
        
        statsDisplay.appendChild(tribeDiv);
    });
    
    statsDisplay.style.display = 'block';
}

// Biome-specific terrain images
const biomeTerrainImages = {
    grass: { terrain: null, trees: null, rocks: null, hills: null },
    snow: { terrain: null, trees: null, rocks: null, hills: null },
    desert: { terrain: null, trees: null, rocks: null, hills: null },
    cemetery: { terrain: null, trees: null, rocks: null, hills: null }
};

const biomeEntityImages = {
    grass: { right: null, left: null },
    snow: { right: null, left: null },
    desert: { right: null, left: null },
    cemetery: { right: null, left: null }
};

const neutralImages = { 4: null }; // Border

// Track entity directions
let entityDirections = {};

let imagesLoaded = 0;
let totalImages = 24; // 4 biomes * 4 terrain + 4 biomes * 2 entity sprites
let useImages = true;

function loadBiomeImage(biome, type, path) {
    const img = new Image();
    img.src = path;
    img.onload = () => {
        biomeTerrainImages[biome][type] = img;
        imagesLoaded++;
        console.log(`Loaded ${biome} ${type}: ${path} (${imagesLoaded}/${totalImages})`);
        if (imagesLoaded === totalImages) {
            console.log('All textures loaded!');
            draw();
        }
    };
    img.onerror = () => {
        console.warn(`Failed to load ${path} - will use solid color`);
        biomeTerrainImages[biome][type] = null;
        imagesLoaded++;
        if (imagesLoaded === totalImages) draw();
    };
}

function loadEntityImage(biome, direction, path) {
    const img = new Image();
    img.src = path;
    img.onload = () => {
        biomeEntityImages[biome][direction] = img;
        imagesLoaded++;
        console.log(`Loaded ${biome} entity ${direction}: ${path} (${imagesLoaded}/${totalImages})`);
        if (imagesLoaded === totalImages) {
            console.log('All textures loaded!');
            draw();
        }
    };
    img.onerror = () => {
        console.warn(`Failed to load ${path} - will use solid color`);
        biomeEntityImages[biome][direction] = null;
        imagesLoaded++;
        if (imagesLoaded === totalImages) draw();
    };
}

// Load snow biome images (north - Norsca)
loadBiomeImage('snow', 'terrain', '/static/northsouth/norscaterrain.png');
loadBiomeImage('snow', 'trees', '/static/northsouth/norscatree.png');
loadBiomeImage('snow', 'rocks', '/static/northsouth/norscarock.png');
loadBiomeImage('snow', 'hills', '/static/northsouth/norscamountain.png');

// Load cemetery biome images (south - Sylvania)
loadBiomeImage('cemetery', 'terrain', '/static/northsouth/sylvaniaterrain.png');
loadBiomeImage('cemetery', 'trees', '/static/northsouth/sylvaniatree.png');
loadBiomeImage('cemetery', 'rocks', '/static/northsouth/sylvaniarock.png');
loadBiomeImage('cemetery', 'hills', '/static/northsouth/sylvaniamountain.png');

loadBiomeImage('grass', 'terrain', '/static/vertical/testgrass2.png');
loadBiomeImage('grass', 'trees', '/static/vertical/grasslandtree.png');
loadBiomeImage('grass', 'rocks', '/static/vertical/grasslandrock.png');
loadBiomeImage('grass', 'hills', '/static/vertical/grasslandmountain.png');

loadBiomeImage('desert', 'terrain', '/static/vertical/redterrain.png');
loadBiomeImage('desert', 'trees', '/static/vertical/deserttree.png');
loadBiomeImage('desert', 'rocks', '/static/vertical/desertrock.png');
loadBiomeImage('desert', 'hills', '/static/vertical/desertmountain.png');

// Load entity sprites
loadEntityImage('snow', 'right', '/static/northsouth/norscaentity1.png');
loadEntityImage('snow', 'left', '/static/northsouth/norscaentity2.png');
loadEntityImage('cemetery', 'right', '/static/northsouth/sylvaniaentity1.png');
loadEntityImage('cemetery', 'left', '/static/northsouth/sylvaniaentity2.png');
loadEntityImage('grass', 'right', '/static/vertical/grasslandentity1.png');
loadEntityImage('grass', 'left', '/static/vertical/grasslandentity2.png');
loadEntityImage('desert', 'right', '/static/vertical/desertentity1.png');
loadEntityImage('desert', 'left', '/static/vertical/desertentity2.png');

function updateViewport() {
    const baseSize = Math.floor(Math.min(canvas.width, canvas.height) / GRID_SIZE);
    CELL_SIZE = Math.floor(baseSize * zoom);
    
    const gridPixelWidth = CELL_SIZE * GRID_SIZE;
    const gridPixelHeight = CELL_SIZE * GRID_SIZE;
    offsetX = Math.floor((canvas.width - gridPixelWidth) / 2) + panOffsetX;
    offsetY = Math.floor((canvas.height - gridPixelHeight) / 2) + panOffsetY;
}

function setTool(tool) {
    // Phase restrictions
    if (mapBuilderPhase === 0 && ![0, 1, 2, 9, 10].includes(tool)) {
        alert('Phase 1: Only terrain painting allowed');
        return;
    }
    if (mapBuilderPhase === 1 && tool !== 4) {
        alert('Phase 2: Only border painting allowed');
        return;
    }
    if (mapBuilderPhase === 2 && ![6, 7, 8, 0].includes(tool)) {
        alert('Phase 3: Only features (trees/rocks/hills) allowed');
        return;
    }
    if (mapBuilderPhase === 3) {
        alert('Phase 4: Configure tribes below, no more painting');
        return;
    }

    currentTool = tool;
    updateToolButtons();
}

function setBrushSize(size) {
    brushSize = size;
    updateBrushButtons();
updatePhaseUI(); // Initialize phase UI on load
}

function toggleFillMode() {
    fillMode = !fillMode;
    const btn = document.getElementById('fillModeBtn');
    if (btn) {
        btn.textContent = `Fill Mode (${fillMode ? 'On' : 'Off'})`;
        if (fillMode) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    }
}

function updateToolButtons() {
    const toolButtons = document.querySelectorAll('[data-tool]');
    toolButtons.forEach(btn => {
        const toolValue = parseInt(btn.getAttribute('data-tool'));
        if (toolValue === currentTool) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
}

function updateBrushButtons() {
    const brushButtons = document.querySelectorAll('[data-brush]');
    brushButtons.forEach(btn => {
        const brushValue = parseInt(btn.getAttribute('data-brush'));
        if (brushValue === brushSize) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });
}

function draw() {
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    ctx.imageSmoothingEnabled = false;

    // Detect entity movements between frames
    if (previousWorld.length === world.length) {
        for (let y = 0; y < GRID_SIZE; y++) {
            for (let x = 0; x < GRID_SIZE; x++) {
                const idx = y * GRID_SIZE + x;
                const cell = world[idx];
                
                if (cell === 3 || cell === 5 || cell === 11 || cell === 12) {
                    const key = `${x},${y}`;
                    
                    let foundDirection = null;
                    
                    // Check left
                    if (x > 0 && previousWorld[y * GRID_SIZE + (x - 1)] === cell && world[y * GRID_SIZE + (x - 1)] !== cell) {
                        foundDirection = 'right';
                    }
                    // Check right
                    else if (x < GRID_SIZE - 1 && previousWorld[y * GRID_SIZE + (x + 1)] === cell && world[y * GRID_SIZE + (x + 1)] !== cell) {
                        foundDirection = 'left';
                    }
                    
                    if (foundDirection) {
                        entityDirections[key] = foundDirection;
                    } else if (!entityDirections[key]) {
                        // Default direction based on tribe/quadrant
                        if (cell === 3) entityDirections[key] = 'right';      // NW
                        else if (cell === 5) entityDirections[key] = 'left';  // NE
                        else if (cell === 11) entityDirections[key] = 'right'; // SW
                        else if (cell === 12) entityDirections[key] = 'left';  // SE
                    }
                }
            }
        }
    }

    // Draw the grid
    for (let y = 0; y < GRID_SIZE; y++) {
        for (let x = 0; x < GRID_SIZE; x++) {
            const cell = world[y * GRID_SIZE + x];
            if (cell === 0) continue;

            let img = null;
            const biome = getBiomeAt(x, y);
            
            // Determine which image to use based on cell type and biome
            if (cell === 1) {
                // Grass terrain (NW quadrant)
                img = biomeTerrainImages.grass.terrain;
            } else if (cell === 2) {
                // Snow terrain (NE quadrant)
                img = biomeTerrainImages.snow.terrain;
            } else if (cell === 9) {
                // Desert terrain (SW quadrant)
                img = biomeTerrainImages.desert.terrain;
            } else if (cell === 10) {
                // Cemetery terrain (SE quadrant)
                img = biomeTerrainImages.cemetery.terrain;
            } else if (cell === 6) {
                // Trees - biome-specific (updates with terrain conversion)
                if (biome === BIOMES.GRASS) {
                    img = biomeTerrainImages.grass.trees;
                } else if (biome === BIOMES.SNOW) {
                    img = biomeTerrainImages.snow.trees;
                } else if (biome === BIOMES.DESERT) {
                    img = biomeTerrainImages.desert.trees;
                } else if (biome === BIOMES.CEMETERY) {
                    img = biomeTerrainImages.cemetery.trees;
                }
            } else if (cell === 7) {
                // Rocks - biome-specific (updates with terrain conversion)
                if (biome === BIOMES.GRASS) {
                    img = biomeTerrainImages.grass.rocks;
                } else if (biome === BIOMES.SNOW) {
                    img = biomeTerrainImages.snow.rocks;
                } else if (biome === BIOMES.DESERT) {
                    img = biomeTerrainImages.desert.rocks;
                } else if (biome === BIOMES.CEMETERY) {
                    img = biomeTerrainImages.cemetery.rocks;
                }
            } else if (cell === 8) {
                // Hills - biome-specific (updates with terrain conversion)
                if (biome === BIOMES.GRASS) {
                    img = biomeTerrainImages.grass.hills;
                } else if (biome === BIOMES.SNOW) {
                    img = biomeTerrainImages.snow.hills;
                } else if (biome === BIOMES.DESERT) {
                    img = biomeTerrainImages.desert.hills;
                } else if (biome === BIOMES.CEMETERY) {
                    img = biomeTerrainImages.cemetery.hills;
                }
            } else if (cell === 3 || cell === 5 || cell === 11 || cell === 12) {
                // Entities - use entityCodeToBiome mapping if available (custom maps)
                const key = `${x},${y}`;
                const direction = entityDirections[key] || 'right';
                
                // Check if we have a custom mapping for this entity code
                let entityBiome = entityCodeToBiome[cell];
                
                // If no custom mapping, fall back to biome-based logic (for preset maps)
                if (!entityBiome) {
                    entityBiome = biome || BIOMES.GRASS;
                }
                
                // Get the appropriate sprite based on entity's tribe biome and direction
                if (entityBiome === BIOMES.GRASS) {
                    img = biomeEntityImages.grass[direction];
                } else if (entityBiome === BIOMES.SNOW) {
                    img = biomeEntityImages.snow[direction];
                } else if (entityBiome === BIOMES.DESERT) {
                    img = biomeEntityImages.desert[direction];
                } else if (entityBiome === BIOMES.CEMETERY) {
                    img = biomeEntityImages.cemetery[direction];
                } else {
                    // Default to grass sprite
                    img = biomeEntityImages.grass[direction];
                }
            } else if (cell === 4) {
                // Border
                img = neutralImages[cell];
            }
            
            if (img && useImages) {
                ctx.drawImage(
                    img,
                    offsetX + x * CELL_SIZE,
                    offsetY + y * CELL_SIZE,
                    CELL_SIZE,
                    CELL_SIZE
                );
            } else {
                // Biome-aware fallback colors
                let color = '#333';
                if (cell === 1) color = '#90EE90';     // Light green for grass
                if (cell === 2) color = '#E0F0FF';     // Light blue/white for snow
                if (cell === 3) color = '#228B22';     // Forest green for grass entity
                if (cell === 4) color = '#0044ff';     // Border
                if (cell === 5) color = '#4A90E2';     // Blue for snow entity
                if (cell === 9) color = '#F4A460';     // Sandy brown for desert
                if (cell === 10) color = '#2C1B3D';    // Dark purple for cemetery
                if (cell === 11) color = '#D2691E';    // Brown for desert entity
                if (cell === 12) color = '#8B0000';    // Dark red for cemetery entity
                
                if (cell === 6) {
                    // Trees: varied by biome
                    if (biome === BIOMES.GRASS) color = '#228B22';      // Forest green
                    else if (biome === BIOMES.SNOW) color = '#1B4D3E';  // Pine green
                    else if (biome === BIOMES.DESERT) color = '#8B7355'; // Cactus brown
                    else if (biome === BIOMES.CEMETERY) color = '#4A3C2F'; // Dead tree brown
                    else color = '#228B22'; // Default
                }
                if (cell === 7) {
                    // Rocks: varied by biome
                    if (biome === BIOMES.GRASS) color = '#808080';      // Gray stone
                    else if (biome === BIOMES.SNOW) color = '#B0C4DE';  // Light ice blue
                    else if (biome === BIOMES.DESERT) color = '#CD853F'; // Desert rock
                    else if (biome === BIOMES.CEMETERY) color = '#696969'; // Gravestone gray
                    else color = '#808080'; // Default
                }
                if (cell === 8) {
                    // Hills: varied by biome
                    if (biome === BIOMES.GRASS) color = '#9ACD32';      // Yellow-green hills
                    else if (biome === BIOMES.SNOW) color = '#C0D6E4';  // Snowy mountains
                    else if (biome === BIOMES.DESERT) color = '#DEB887'; // Sand dunes
                    else if (biome === BIOMES.CEMETERY) color = '#534D56'; // Fog/mist
                    else color = '#9ACD32'; // Default
                }

                ctx.fillStyle = color;
                ctx.fillRect(offsetX + x * CELL_SIZE, offsetY + y * CELL_SIZE, CELL_SIZE, CELL_SIZE);
            }
        }
    }
    
    // Store current world as previous for next frame
    previousWorld = [...world];

    drawPhaseIndicator();
}

function drawPhaseIndicator() {
    const phases = ['Paint Terrain', 'Draw Borders', 'Add Features', 'Configure Tribes', 'Play'];
    ctx.fillStyle = 'rgba(0, 0, 0, 0.7)';
    ctx.fillRect(10, 10, 250, 50);
    ctx.fillStyle = 'white';
    ctx.font = '16px Arial';
    ctx.fillText(`Phase ${mapBuilderPhase + 1}: ${phases[mapBuilderPhase]}`, 20, 35)
}

// WebSocket connection
const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);
ws.binaryType = 'arraybuffer';
ws.onopen = () => console.log('WS connected');
ws.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
        const data = new Uint8Array(event.data);
        world = Array.from(data);
    } else {
        let msg;
        try {
            msg = JSON.parse(event.data);
        } catch (e) {
            console.error('[WS] Invalid JSON received:', event.data, e);
            return;
        }

        if (msg.action === 'inspect_response') {
            // Only allow inspect if map is finalized (phase 4)
            if (mapBuilderPhase !== 4) {
                return;
            }
            
            const popup = document.getElementById('inspectPopup');
            if (msg.empty) {
                popup.style.display = 'none';
            } else {
                document.getElementById('entityName').innerText = msg.name || 'Unknown Entity';
                
                // Display rank above health
                let rankText = msg.rank || 'Base';
                if (msg.rankDamage > 0 || msg.rankArmor > 0) {
                    const bonuses = [];
                    if (msg.rankDamage > 0) bonuses.push(`+${msg.rankDamage} dmg`);
                    if (msg.rankArmor > 0) bonuses.push(`+${msg.rankArmor} armor`);
                    rankText += ` (${bonuses.join(', ')})`;
                }
                document.getElementById('entityRank').innerText = rankText;
                
                document.getElementById('entityHealth').innerText = msg.health || 100;
                
                // Display weapon with damage
                let weaponText = msg.weapon || 'None';
                if (msg.weapon && msg.weapon !== 'None') {
                    const weaponDmg = msg.weapon === 'Wood Sword' ? 3 : 4;
                    weaponText = `${msg.weapon} (+${weaponDmg} dmg)`;
                }
                document.getElementById('entityWeapon').innerText = weaponText;
                
                // Display armor with defense
                let armorText = msg.armor || 'None';
                if (msg.armor && msg.armor !== 'None') {
                    const armorDef = msg.armor === 'Wood Armor' ? 2 : 3;
                    armorText = `${msg.armor} (+${armorDef} def)`;
                }
                document.getElementById('entityArmor').innerText = armorText;
                
                // Display total damage (including racial passive + rank)
                let damageText = `${msg.damage || 5}`;
                const bonusParts = [];
                if (msg.racialDamage && msg.racialDamage > 0) {
                    bonusParts.push(`Racial: +${msg.racialDamage}`);
                }
                if (msg.rankDamage && msg.rankDamage > 0) {
                    bonusParts.push(`Rank: +${msg.rankDamage}`);
                }
                if (bonusParts.length > 0) {
                    damageText += ` (${bonusParts.join(', ')})`;
                }
                document.getElementById('entityDamage').innerText = damageText;
                
                // Display defense with breakdown (armor + rank + racial)
                let defenseText = `${msg.defense || 0}`;
                const defenseBonusParts = [];
                if (msg.armor && msg.armor !== 'None') {
                    const armorDef = msg.armor === 'Wood Armor' ? 2 : 3;
                    defenseBonusParts.push(`Armor: +${armorDef}`);
                }
                if (msg.racialDefense && msg.racialDefense > 0) {
                    defenseBonusParts.push(`Racial: +${msg.racialDefense}`);
                }
                if (msg.rankArmor && msg.rankArmor > 0) {
                    defenseBonusParts.push(`Rank: +${msg.rankArmor}`);
                }
                if (defenseBonusParts.length > 0) {
                    defenseText += ` (${defenseBonusParts.join(', ')})`;
                }
                document.getElementById('entityDefense').innerText = defenseText;
                
                // Display evasion (racial passive)
                let evasionText = `${msg.evasion || 0}%`;
                if (msg.evasion && msg.evasion > 0) {
                    evasionText += ' (Racial)';
                }
                document.getElementById('entityEvasion').innerText = evasionText;
                
                popup.style.left = (lastClickX + 15) + 'px';
                popup.style.top = (lastClickY + 15) + 'px';
                popup.style.display = 'block';
            }
        } 
        
        // ===== NEW: CUSTOM MAP INITIALIZED =====
        else if (msg.action === 'custom_map_initialized') {
            console.log('✅ Custom map initialized successfully!');
            
            // Store tribe configurations
            if (msg.tribes) {
                activeTribeConfigs = msg.tribes;
                
                // Build entityCode -> biome mapping
                const vizCodeToBiomeMap = {
                    3: BIOMES.GRASS,      // Wanderers (EntityVizCode 3)
                    5: BIOMES.SNOW,       // Norsca (EntityVizCode 5)
                    11: BIOMES.DESERT,    // Nomads (EntityVizCode 11)
                    12: BIOMES.CEMETERY   // Sylvania (EntityVizCode 12)
                };
                
                entityCodeToBiome = {};
                Object.values(msg.tribes).forEach(tribe => {
                    const vizCode = tribe.entityVizCode;
                    entityCodeToBiome[vizCode] = vizCodeToBiomeMap[vizCode] || BIOMES.GRASS;
                });
                
                console.log('Entity code to biome mapping:', entityCodeToBiome);
                
                // Update stats display with only active tribes
                updateStatsDisplay(msg.tribes);
                
                // Store tribe names for later use
                Object.entries(msg.tribes).forEach(([tribeId, tribe]) => {
                    tribeNames[tribeId] = tribe.name;
                });
            }
            
            // Advance to phase 4
            mapBuilderPhase = 4;
            updatePhaseUI();
            
            // Show controls
            document.getElementById('leftControls').style.display = 'block';
            
            alert('Map finalized! You can now start the war.');
        } 
        
        // ===== STATS UPDATE =====
        else if (msg.action === 'stats_update' || msg.tribes) {
            // Only process stats if map is finalized
            if (mapBuilderPhase === 4) {
                if (msg.tribes) {
                    // Update tribe names if provided
                    for (let tribeID in msg.tribes) {
                        const tribeData = msg.tribes[tribeID];
                        if (tribeData.name) {
                            tribeNames[tribeID] = tribeData.name;
                        }
                    }
                    
                    // Update stats for each tribe
                    Object.entries(msg.tribes).forEach(([tribeId, stats]) => {
                        const countElem = document.getElementById(`tribe${tribeId}Count`);
                        const woodElem = document.getElementById(`tribe${tribeId}Wood`);
                        const stoneElem = document.getElementById(`tribe${tribeId}Stone`);
                        
                        if (countElem) countElem.textContent = stats.count || 0;
                        if (woodElem) woodElem.textContent = stats.wood || 0;
                        if (stoneElem) stoneElem.textContent = stats.stone || 0;
                    });
                } else {
                    console.warn('[WS] Stats message missing tribes field:', msg);
                }

                // Update speed controls
                if (msg.speed !== undefined) {
                    stats.speed = msg.speed;
                    document.getElementById('speedDisplay').textContent = `${msg.speed}x`;
                    document.getElementById('plus').disabled = msg.speed >= maxSpeed;
                    document.getElementById('minus').disabled = msg.speed <= minSpeed;
                }
                
                // Update pause state
                if (msg.paused !== undefined) {
                    stats.paused = msg.paused;
                    document.getElementById('pause').textContent = msg.paused ? 'Resume' : 'Pause';
                }

                // Handle victory
                if (msg.winner) {
                    let winnerName = tribeNames[msg.winner] || `Tribe ${msg.winner}`;
                    if (msg.winner === "draw") {
                        winnerName = "Draw";
                    }
                    document.getElementById('victoryMessage').style.display = 'block';
                    document.getElementById('victoryMessage').textContent = `${winnerName} Wins!`;
                } else {
                    document.getElementById('victoryMessage').style.display = 'none';
                }
            }
        }
        
        // ===== CUSTOM MAP ERROR =====
        else if (msg.action === 'custom_map_error') {
            console.error('Custom map initialization failed:', msg.error);
            alert('Failed to initialize custom map: ' + (msg.error || 'Unknown error'));
            // Go back to tribe config phase
            mapBuilderPhase = 3;
            updatePhaseUI();
        }
    }

    draw();
};

ws.onclose = () => console.log('WS closed');

// ===== MOUSE CONTROLS =====
canvas.addEventListener('mousedown', (e) => {
    if (e.button === 0 && e.shiftKey) {
        isPanning = true;
        panStartX = e.clientX - panOffsetX;
        panStartY = e.clientY - panOffsetY;
        canvas.style.cursor = 'grabbing';
        return;
    }
    
    if (e.button === 0) {
        isDrawing = true;
        tryPlace(e);
    }
});

canvas.addEventListener('mousemove', (e) => {
    if (isPanning) {
        panOffsetX = e.clientX - panStartX;
        panOffsetY = e.clientY - panStartY;
        updateViewport();
        draw();
    } else if (isDrawing) {
        tryPlace(e);
    } else if (e.shiftKey) {
        canvas.style.cursor = 'grab';
    } else {
        canvas.style.cursor = 'crosshair';
    }
});

canvas.addEventListener('mouseup', () => {
    if (isPanning) {
        isPanning = false;
        canvas.style.cursor = 'crosshair';
    }
    isDrawing = false;
});

canvas.addEventListener('mouseleave', () => {
    isPanning = false;
    isDrawing = false;
    canvas.style.cursor = 'crosshair';
});

// RIGHT-CLICK for inspect
canvas.addEventListener('contextmenu', (e) => {
    e.preventDefault();
    
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left - offsetX;
    const mouseY = e.clientY - rect.top - offsetY;
    const gridX = Math.floor(mouseX / CELL_SIZE);
    const gridY = Math.floor(mouseY / CELL_SIZE);

    if (gridX >= 0 && gridX < GRID_SIZE && gridY >= 0 && gridY < GRID_SIZE) {
        ws.send(JSON.stringify({ action: 'inspect', x: gridX, y: gridY }));
        lastClickX = e.clientX;
        lastClickY = e.clientY;
    } else {
        document.getElementById('inspectPopup').style.display = 'none';
    }
});

// Close inspect popup on click or ESC
document.addEventListener('click', (e) => {
    const popup = document.getElementById('inspectPopup');
    if (popup && popup.style.display === 'block') {
        popup.style.display = 'none';
    }
});

document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
        const popup = document.getElementById('inspectPopup');
        if (popup) {
            popup.style.display = 'none';
        }
    }
    
    if (e.key === '+' || e.key === '=') {
        e.preventDefault();
        zoom = Math.min(zoom + ZOOM_SPEED, MAX_ZOOM);
        updateViewport();
        draw();
        const zoomDisplay = document.getElementById('zoomDisplay');
        if (zoomDisplay) zoomDisplay.textContent = `${(zoom * 100).toFixed(0)}%`;
    }
    if (e.key === '-' || e.key === '_') {
        e.preventDefault();
        zoom = Math.max(zoom - ZOOM_SPEED, MIN_ZOOM);
        updateViewport();
        draw();
        const zoomDisplay = document.getElementById('zoomDisplay');
        if (zoomDisplay) zoomDisplay.textContent = `${(zoom * 100).toFixed(0)}%`;
    }
    if (e.key === '0') {
        e.preventDefault();
        zoom = 1.0;
        panOffsetX = 0;
        panOffsetY = 0;
        updateViewport();
        draw();
        const zoomDisplay = document.getElementById('zoomDisplay');
        if (zoomDisplay) zoomDisplay.textContent = '100%';
    }
    
    const PAN_STEP = 50;
    if (e.key === 'ArrowLeft') {
        panOffsetX += PAN_STEP;
        updateViewport();
        draw();
    }
    if (e.key === 'ArrowRight') {
        panOffsetX -= PAN_STEP;
        updateViewport();
        draw();
    }
    if (e.key === 'ArrowUp') {
        panOffsetY += PAN_STEP;
        updateViewport();
        draw();
    }
    if (e.key === 'ArrowDown') {
        panOffsetY -= PAN_STEP;
        updateViewport();
        draw();
    }
});

// ===== ZOOM CONTROLS =====
canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    
    const zoomIn = e.deltaY < 0;
    const oldZoom = zoom;
    
    if (zoomIn) {
        zoom = Math.min(zoom + ZOOM_SPEED, MAX_ZOOM);
    } else {
        zoom = Math.max(zoom - ZOOM_SPEED, MIN_ZOOM);
    }
    
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left;
    const mouseY = e.clientY - rect.top;
    
    const zoomRatio = zoom / oldZoom;
    panOffsetX = mouseX - (mouseX - panOffsetX) * zoomRatio - (canvas.width / 2 - panOffsetX) * (zoomRatio - 1);
    panOffsetY = mouseY - (mouseY - panOffsetY) * zoomRatio - (canvas.height / 2 - panOffsetY) * (zoomRatio - 1);
    
    updateViewport();
    draw();
    
    const zoomDisplay = document.getElementById('zoomDisplay');
    if (zoomDisplay) {
        zoomDisplay.textContent = `${(zoom * 100).toFixed(0)}%`;
    }
});

function tryPlace(e) {
    const now = Date.now();
    if (now - lastPlaceTime < PLACE_DELAY) return;
    const rect = canvas.getBoundingClientRect();
    const mouseX = e.clientX - rect.left - offsetX;
    const mouseY = e.clientY - rect.top - offsetY;
    const centerX = Math.floor(mouseX / CELL_SIZE);
    const centerY = Math.floor(mouseY / CELL_SIZE);
    
    if (fillMode) {
        if (currentTool !== 1 && currentTool !== 2) {
            console.warn('Fill only supported for red(1)/blue(2)');
            return;
        }
        floodFill(centerX, centerY, currentTool);
    } else {
        const radius = Math.floor(brushSize / 2);
        const places = [];
        for (let dy = -radius; dy <= radius; dy++) {
            for (let dx = -radius; dx <= radius; dx++) {
                const x = centerX + dx;
                const y = centerY + dy;
                if (x >= 0 && x < GRID_SIZE && y >= 0 && y < GRID_SIZE) {
                    places.push({x, y});
                }
            }
        }
        if (places.length > 0) {
            ws.send(JSON.stringify({ action: 'place_batch', places: places.map(p => ({x: p.x, y: p.y, type: currentTool})) }));
        }
    }
    lastPlaceTime = now;
}

function updateUI() {
    document.getElementById('speedDisplay').textContent = `${stats.speed}x`;
    document.getElementById('pause').textContent = stats.paused ? 'Resume' : 'Pause';
    document.getElementById('plus').disabled = stats.speed >= maxSpeed;
    document.getElementById('minus').disabled = stats.speed <= minSpeed;
}

document.getElementById('plus').addEventListener('click', () => changeSpeed(1));
document.getElementById('minus').addEventListener('click', () => changeSpeed(-1));
document.getElementById('pause').addEventListener('click', () => {
    ws.send(JSON.stringify({ action: 'toggle_pause' }));
});

function changeSpeed(delta) {
    let currentIndex = speeds.indexOf(stats.speed);
    if (currentIndex === -1) currentIndex = 2;
    let newIndex = currentIndex + delta;
    if (newIndex < 0) newIndex = 0;
    if (newIndex >= speeds.length) newIndex = speeds.length - 1;
    let newSpeed = speeds[newIndex];
    ws.send(JSON.stringify({ action: 'set_speed', multiplier: newSpeed }));
}

window.addEventListener('resize', () => {
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
    updateViewport();
    draw();
});

updateUI();
updateToolButtons();
updateBrushButtons();

function floodFill(startX, startY, fillType) {
    const startIdx = startY * GRID_SIZE + startX;
    const targetType = world[startIdx];
    if (targetType !== 0 && targetType !== 1 && targetType !== 2) {
        console.warn('Fill start not terrain (1/2) - value:', targetType);
        return;
    }
    const queue = [[startX, startY]];
    let filledCount = 0;
    const visited = new Set();
    const places = [];
    while (queue.length > 0 && filledCount < 5000) {
        const [x, y] = queue.shift();
        const key = `${x},${y}`;
        if (visited.has(key)) continue;
        visited.add(key);
        const idx = y * GRID_SIZE + x;
        if (world[idx] === targetType) {
            places.push({x, y});
            filledCount++;
            if (x > 0) queue.push([x - 1, y]);
            if (x < GRID_SIZE - 1) queue.push([x + 1, y]);
            if (y > 0) queue.push([x, y - 1]);
            if (y < GRID_SIZE - 1) queue.push([x, y + 1]);
        }
    }
    console.log('Found', filledCount, 'cells to fill');
    if (filledCount >= 5000) {
        console.warn('Fill limit reached—large area truncated');
    }
    const batchSize = 500;
    let i = 0;
    function sendBatch() {
        const batch = places.slice(i, i + batchSize);
        if (batch.length === 0) return;
        ws.send(JSON.stringify({ action: 'place_batch', places: batch.map(p => ({x: p.x, y: p.y, type: fillType})) }));
        i += batchSize;
        setTimeout(sendBatch, 100);
    }
    sendBatch();
}

document.getElementById('startWar').addEventListener('click', () => {
    ws.send(JSON.stringify({ action: 'start_war' }));
    document.getElementById('startWar').disabled = true;
    document.getElementById('startWar').textContent = 'War Started';
});

function resetWorld() {
    if (confirm('Reset the world? This will clear everything and restart fresh.')) {
        ws.send(JSON.stringify({ action: 'reset' }));
        document.getElementById('startWar').disabled = false;
        document.getElementById('startWar').textContent = 'Start War';
        document.getElementById('victoryMessage').style.display = 'none';
        
        entityDirections = {};
        previousWorld = [];
    }
}

function nextPhase() {
    if (mapBuilderPhase === 0) {
        // Validate: check if any terrain painted
        const hasRed = world.includes(1);
        const hasBlue = world.includes(2);
        const hasYellow = world.includes(9);
        const hasGreen = world.includes(10);
        
        if (!hasRed && !hasBlue && !hasYellow && !hasGreen) {
            alert('Please paint at least one terrain type!');
            return;
        }
    }
    
    if (mapBuilderPhase === 1) {
        // Validate: check if borders exist (optional)
        const hasBorder = world.includes(4);
        if (!hasBorder) {
            if (!confirm('No borders drawn. Continue anyway?')) {
                return;
            }
        }
    }
    
    mapBuilderPhase++;
    updatePhaseUI();
    
    if (mapBuilderPhase === 3) {
        showTribeConfigPanel();
    }
}

function previousPhase() {
    if (mapBuilderPhase > 0) {
        mapBuilderPhase--;
        updatePhaseUI();
        
        if (mapBuilderPhase < 3) {
            document.getElementById('tribeConfigPanel').style.display = 'none';
        }
    }
}

function updatePhaseUI() {
    const nextBtn = document.getElementById('nextPhaseBtn');
    const prevBtn = document.getElementById('prevPhaseBtn');
    const toolsPanel = document.getElementById('rightToolbar');
    const phaseIndicator = document.getElementById('phaseIndicator');
    const autoBorderBtn = document.getElementById('autoBorderBtn');
    
    // Get all tool buttons
    const toolButtons = {
        0: document.querySelector('[data-tool="0"]'),   // Erase
        1: document.querySelector('[data-tool="1"]'),   // Red
        2: document.querySelector('[data-tool="2"]'),   // Blue
        4: document.querySelector('[data-tool="4"]'),   // Border
        6: document.querySelector('[data-tool="6"]'),   // Trees
        7: document.querySelector('[data-tool="7"]'),   // Rocks
        8: document.querySelector('[data-tool="8"]'),   // Hills
        9: document.querySelector('[data-tool="9"]'),   // Yellow
        10: document.querySelector('[data-tool="10"]'), // Green
    };
    
    // Hide all tools first
    Object.values(toolButtons).forEach(btn => {
        if (btn) btn.style.display = 'none';
    });
    
    // Hide auto-border button by default
    if (autoBorderBtn) autoBorderBtn.style.display = 'none';
    
    // Show tools based on current phase
    switch(mapBuilderPhase) {
        case 0: // Phase 1: Paint Terrain
            phaseIndicator.textContent = 'Phase 1: Paint Terrain';
            if (toolButtons[0]) toolButtons[0].style.display = 'block';  // Erase
            if (toolButtons[1]) toolButtons[1].style.display = 'block';  // Red
            if (toolButtons[2]) toolButtons[2].style.display = 'block';  // Blue
            if (toolButtons[9]) toolButtons[9].style.display = 'block';  // Yellow
            if (toolButtons[10]) toolButtons[10].style.display = 'block'; // Green
            toolsPanel.style.display = 'block';
            nextBtn.style.display = 'block';
            prevBtn.style.display = 'block';
            nextBtn.textContent = 'Next Phase →';
            break;
            
        case 1: // Phase 2: Paint Borders
            phaseIndicator.textContent = 'Phase 2: Borders (Auto-Generate or Draw)';
            if (toolButtons[0]) toolButtons[0].style.display = 'block';  // Erase
            if (toolButtons[4]) toolButtons[4].style.display = 'block';  // Border
            if (autoBorderBtn) autoBorderBtn.style.display = 'block';    // Auto-Border Button
            toolsPanel.style.display = 'block';
            nextBtn.style.display = 'block';
            prevBtn.style.display = 'block';
            nextBtn.textContent = 'Next Phase →';
            break;
            
        case 2: // Phase 3: Add Features
            phaseIndicator.textContent = 'Phase 3: Add Features (Trees, Rocks, Hills)';
            if (toolButtons[0]) toolButtons[0].style.display = 'block';  // Erase
            if (toolButtons[6]) toolButtons[6].style.display = 'block';  // Trees
            if (toolButtons[7]) toolButtons[7].style.display = 'block';  // Rocks
            if (toolButtons[8]) toolButtons[8].style.display = 'block';  // Hills
            toolsPanel.style.display = 'block';
            nextBtn.style.display = 'block';
            prevBtn.style.display = 'block';
            nextBtn.textContent = 'Next Phase →';
            break;
            
        case 3: // Phase 4: Configure Tribes
            phaseIndicator.textContent = 'Phase 4: Configure Tribes';
            toolsPanel.style.display = 'none';
            nextBtn.style.display = 'block';
            prevBtn.style.display = 'block';
            nextBtn.textContent = 'Finalize Map';
            break;
            
        case 4: // Phase 5: Simulation Running
            phaseIndicator.textContent = 'Simulation Running';
            nextBtn.style.display = 'none';
            prevBtn.style.display = 'none';
            document.getElementById('tribeConfigPanel').style.display = 'none';
            toolsPanel.style.display = 'block';
            
            // Show all tools in simulation mode
            Object.values(toolButtons).forEach(btn => {
                if (btn) btn.style.display = 'block';
            });
            
            // Show the start war button and reset button
            const startWarBtn = document.getElementById('startWar');
            const resetBtn = document.querySelector('[onclick="resetWorld()"]');
            if (startWarBtn) {
                startWarBtn.style.display = 'block';
                startWarBtn.disabled = false;
            }
            if (resetBtn) resetBtn.style.display = 'block';
            
            // Show left controls and stats
            const leftControls = document.getElementById('leftControls');
            const statsDisplay = document.getElementById('statsDisplay');
            if (leftControls) leftControls.style.display = 'block';
            if (statsDisplay) statsDisplay.style.display = 'block';
            
            // Hide phase controls
            const phaseControls = document.getElementById('phaseControls');
            if (phaseControls) phaseControls.style.display = 'none';
            break;
    }
    
    prevBtn.disabled = mapBuilderPhase === 0;
}

function showTribeConfigPanel() {
    const usedTerrains = new Set();
    for (let cell of world) {
        if (cell === 1 || cell === 2 || cell === 9 || cell === 10) {
            usedTerrains.add(cell);
        }
    }
    
    if (usedTerrains.size === 0) {
        alert('No terrain found! Go back and paint some terrain.');
        previousPhase();
        return;
    }
    
    const panel = document.getElementById('tribeConfigPanel');
    panel.style.display = 'block';
    panel.innerHTML = '<h2>Configure Tribes</h2>';
    
    const terrainNames = {
        1: 'Red/Grass',
        2: 'Blue/Snow', 
        9: 'Yellow/Desert',
        10: 'Green/Cemetery'
    };
    
    const sortedTerrains = Array.from(usedTerrains).sort();
    
    sortedTerrains.forEach(terrainType => {
        panel.innerHTML += `
            <div class="tribe-config" style="margin: 15px 0; padding: 10px; background: rgba(255,255,255,0.1); border-radius: 5px;">
                <h3 style="margin: 5px 0;">${terrainNames[terrainType]} Terrain</h3>
                <select id="tribe-${terrainType}" style="width: 100%; padding: 5px; font-size: 14px;">
                    <option value="none">Unused/Neutral</option>
                    <option value="Wanderers">Wanderers (Elves - 22% Evasion)</option>
                    <option value="Norsca">Norsca (Vikings - +2 Defense)</option>
                    <option value="Nomads">Nomads (Orcs - +2 Damage)</option>
                    <option value="Sylvania">Sylvania (Vampires - +1 Dmg, 12% Evasion)</option>
                </select>
            </div>
        `;
    });
    
    panel.innerHTML += `
        <button onclick="previousPhase()" style="margin-top: 10px;">Back</button>
        <button onclick="finalizeCustomMap()" style="margin-top: 10px;">Finalize & Start</button>
    `;
}

function finalizeCustomMap() {
    // Collect assignments
    const assignments = {};
    let hasAtLeastOne = false;
    
    [1, 2, 9, 10].forEach(terrainType => {
        const select = document.getElementById(`tribe-${terrainType}`);
        if (select && select.value !== 'none') {
            assignments[terrainType.toString()] = select.value;
            hasAtLeastOne = true;
        }
    });
    
    if (!hasAtLeastOne) {
        alert('Please assign at least one tribe to a terrain!');
        return;
    }
    
    // Send to backend
    ws.send(JSON.stringify({
        action: 'init_custom_map',
        terrain: world,
        tribeAssignments: assignments
    }));
    
    mapBuilderPhase = 4;
    updatePhaseUI();
}

function resetBuilder() {
    if (confirm('Reset the entire map builder? This will clear everything.')) {
        mapBuilderPhase = 0;
        world = new Array(GRID_SIZE * GRID_SIZE).fill(0);
        previousWorld = [];
        entityDirections = {};
        updatePhaseUI();
        document.getElementById('tribeConfigPanel').style.display = 'none';
        draw();
    }
}

function autoGenerateBorders() {
    const borderCells = [];
    
    for (let y = 0; y < GRID_SIZE; y++) {
        for (let x = 0; x < GRID_SIZE; x++) {
            const idx = y * GRID_SIZE + x;
            const currentTerrain = world[idx];
            
            // Skip if not terrain (don't overwrite features/borders)
            if (currentTerrain !== 1 && currentTerrain !== 2 && currentTerrain !== 9 && currentTerrain !== 10) {
                continue;
            }
            
            // Check all 4 neighbors
            let hasDifferentNeighbor = false;
            
            // Check left
            if (x > 0) {
                const leftIdx = y * GRID_SIZE + (x - 1);
                const leftTerrain = world[leftIdx];
                if ((leftTerrain === 1 || leftTerrain === 2 || leftTerrain === 9 || leftTerrain === 10) && leftTerrain !== currentTerrain) {
                    hasDifferentNeighbor = true;
                }
            }
            
            // Check right
            if (x < GRID_SIZE - 1) {
                const rightIdx = y * GRID_SIZE + (x + 1);
                const rightTerrain = world[rightIdx];
                if ((rightTerrain === 1 || rightTerrain === 2 || rightTerrain === 9 || rightTerrain === 10) && rightTerrain !== currentTerrain) {
                    hasDifferentNeighbor = true;
                }
            }
            
            // Check up
            if (y > 0) {
                const upIdx = (y - 1) * GRID_SIZE + x;
                const upTerrain = world[upIdx];
                if ((upTerrain === 1 || upTerrain === 2 || upTerrain === 9 || upTerrain === 10) && upTerrain !== currentTerrain) {
                    hasDifferentNeighbor = true;
                }
            }
            
            // Check down
            if (y < GRID_SIZE - 1) {
                const downIdx = (y + 1) * GRID_SIZE + x;
                const downTerrain = world[downIdx];
                if ((downTerrain === 1 || downTerrain === 2 || downTerrain === 9 || downTerrain === 10) && downTerrain !== currentTerrain) {
                    hasDifferentNeighbor = true;
                }
            }
            
            // If this cell borders a different terrain, mark for border placement
            if (hasDifferentNeighbor) {
                borderCells.push({x: x, y: y, type: 4}); // Border type is 4
                world[idx] = 4; // Update local world immediately for visual feedback
            }
        }
    }
    
    console.log(`Auto-generating ${borderCells.length} border cells`);
    
    // Send to backend in batches
    if (borderCells.length > 0) {
        const batchSize = 500;
        let i = 0;
        
        function sendBatch() {
            const batch = borderCells.slice(i, i + batchSize);
            if (batch.length === 0) {
                console.log('All borders sent to backend');
                return;
            }
            
            ws.send(JSON.stringify({ 
                action: 'place_batch', 
                places: batch 
            }));
            
            i += batchSize;
            setTimeout(sendBatch, 100);
        }
        
        sendBatch();
        draw(); // Redraw immediately for visual feedback
    } else {
        alert('No borders to generate - make sure you have at least 2 different terrain types adjacent to each other!');
    }
    
    return borderCells.length;
}