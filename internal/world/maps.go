package world

import (
	"log"
	"math/rand"
	"strconv"
	"sync/atomic"
)

var TribeNamePool = []string{
    "Ice Kingdom",
	"Fire Empire",
	"Storm Clan",
	"Earth Realm",
	"Shadow Legion",
	"Light Order",
	"Desert Nomads",
	"Forest Tribe",
	"Mountain Folk",
	"Ocean Raiders",
	"Sky Warriors",
	"Crystal Dynasty",
}

func GetRandomTribeNames(count int) []string {
    if count > len(TribeNamePool) {
        log.Printf("Warning: Requested %d tribe names but only %d available", count, len(TribeNamePool))
        count = len(TribeNamePool)
    }

    poolyCopy := make([]string, len(TribeNamePool))
    copy(poolyCopy, TribeNamePool)

    rand.Shuffle(len(poolyCopy), func(i, j int) {
        poolyCopy[i], poolyCopy[j] = poolyCopy[j], poolyCopy[i]
    })

    return poolyCopy[:count]
}

func (w *World) ConvertBordersToTerrain() {
	w.Mu.Lock()
	defer w.Mu.Unlock()
	
	converted := 0
	
	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			idx := y*GridSize + x
			
			// Only convert border cells
			if w.Terrain[idx] != uint8(TerrainBorder) {
				continue
			}
			
			// Collect adjacent terrain types (not borders, not empty)
			adjacentTerrains := make(map[uint8]int)
			
			// Check all 4 neighbors
			directions := [][2]int{
				{-1, 0}, // left
				{1, 0},  // right
				{0, -1}, // up
				{0, 1},  // down
			}
			
			for _, dir := range directions {
				nx := x + dir[0]
				ny := y + dir[1]
				
				// Bounds check
				if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
					nidx := ny*GridSize + nx
					neighborTerrain := w.Terrain[nidx]
					
					// Count terrain types (1=red, 2=blue, 9=yellow, 10=green)
					if neighborTerrain == 1 || neighborTerrain == 2 || 
					   neighborTerrain == 9 || neighborTerrain == 10 {
						adjacentTerrains[neighborTerrain]++
					}
				}
			}
			
			// Convert to the most common adjacent terrain
			if len(adjacentTerrains) > 0 {
				maxCount := 0
				var chosenTerrain uint8 = 1 // default to red if tie
				
				for terrain, count := range adjacentTerrains {
					if count > maxCount {
						maxCount = count
						chosenTerrain = terrain
					}
				}
				
				w.Terrain[idx] = chosenTerrain
				converted++
			}
		}
	}
	
	log.Printf("Converted %d border cells to adjacent terrain", converted)
}

// Classic left/right vertical split (exact copy of your old InitSplitHalves logic)
func InitVerticalSplit(w *World) {
    for i := range w.Terrain {
        w.Terrain[i] = 0
    }

    // Set active tribes config
    w.Tribes = map[uint8]TribeConfig{
        1: {
            HomeTerrain: TerrainRed, 
            EntityVizCode: 3, 
            Starters: 20,
            Name: "Nomads",
            DamageBonus: 2, // Orcs get 2 dmg racial
            BaseEvasion: 0.0, // No evasion
            DefenseBonus: 0, // No defense racial
        },

        2: {
            HomeTerrain: TerrainBlue, 
            EntityVizCode: 5, 
            Starters: 20,
            Name: "Wanderers",
            DamageBonus: 0, // no damage racial
            BaseEvasion: 0.22, // 22% evasion
            DefenseBonus: 0, // no defense racial
        },
    }

    // randomNames := GetRandomTribeNames(2)
    // w.Tribes = map[uint8]TribeConfig{
    //     1: {
    //         HomeTerrain: TerrainRed, 
    //         EntityVizCode: 3, 
    //         Starters: 20,
    //         Name: randomNames[0],
    //     },
    //     2: {
    //         HomeTerrain: TerrainBlue, 
    //         EntityVizCode: 5, 
    //         Starters: 20,
    //         Name: randomNames[1],
    //     },
    // }

    // Paint base terrain
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            if x < 50 {
                w.Terrain[y*GridSize + x] = uint8(TerrainRed)
            } else if x > 50 {
                w.Terrain[y*GridSize + x] = uint8(TerrainBlue)
            }
        }
    }

    // Center green border
    for y := 0; y < GridSize; y++ {
        w.Terrain[y*GridSize + 50] = uint8(TerrainBorder)
    }

    // Helper: list of all home flats for this map
    homeFlats := make(map[TerrainType]bool)
    for _, cfg := range w.Tribes {
        homeFlats[cfg.HomeTerrain] = true
    }

    isHomeFlat := func(t TerrainType) bool {
        return homeFlats[t]
    }

    // === Trees (25 forests, 12-20 trees each) ===
    for i := 0; i < 25; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i-- // Retry
            continue
        }

        treesInCluster := 12 + rand.Intn(9)
        for j := 0; j < treesInCluster; j++ {
            nx, ny := cx + rand.Intn(15)-7, cy + rand.Intn(15)-7
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainTrees)
                    }
                }
            }
        }
    }

    // === Rocks (15 clusters, 8-15 each, tighter) ===
    for i := 0; i < 15; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        rocksInCluster := 8 + rand.Intn(8)
        for j := 0; j < rocksInCluster; j++ {
            nx, ny := cx + rand.Intn(11)-5, cy + rand.Intn(11)-5
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainRocks)
                    }
                }
            }
        }
    }

    // === Hills (20 areas, 20-40 each, larger) ===
    for i := 0; i < 20; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        hillsInArea := 20 + rand.Intn(21)
        for j := 0; j < hillsInArea; j++ {
            nx, ny := cx + rand.Intn(21)-10, cy + rand.Intn(21)-10
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainHills)
                    }
                }
            }
        }
    }

    // Starters (generic)
    for tribe, cfg := range w.Tribes {
        for i := 0; i < cfg.Starters; i++ {
            placed := false
            for attempts := 0; attempts < 1000; attempts++ {
                x, y := rand.Intn(GridSize), rand.Intn(GridSize)
                if w.Entities[y][x] == nil && TerrainType(w.Terrain[y*GridSize + x]) == cfg.HomeTerrain {
                    counter := w.nextEntityID[tribe]
                    if counter == nil {
                        counter = new(uint32)
                        w.nextEntityID[tribe] = counter
                    }
                    id := atomic.AddUint32(counter, 1)
                    w.Entities[y][x] = &Entity{
                        Health: 100, 
                        Tribe: tribe, 
                        Weapon: WeaponNone, 
                        Armor: ArmorNone, 
                        ID: id,
                        Evasion: cfg.BaseEvasion,
                        Rank: RankBase,
                    }
                    placed = true
                    break
                }
            }

            if !placed {
                log.Printf("Warning: Could not place starter for tribe %d (vertical map)", tribe)
            }
        }
    }
}

// Placeholder for future maps — copy pattern
func InitFourQuadrants(w *World) {
    // Define 4 tribes + home terrains (new TerrainType consts needed)
    //randomNames := GetRandomTribeNames(4)

    for i := range w.Terrain {
        w.Terrain[i] = 0
    }

    w.Tribes = map[uint8]TribeConfig{
        1: {
            HomeTerrain: TerrainRed,
            EntityVizCode: 3,
            Starters: 20,
            Name: "Wanderers",
            DamageBonus: 0,
            BaseEvasion: 0.22,
            DefenseBonus: 0, 
        },

        2: {
            HomeTerrain: TerrainBlue,
            EntityVizCode: 5,
            Starters: 20,
            Name: "Norsca",
            DamageBonus: 0,
            BaseEvasion: 0.00,
            DefenseBonus: 2, 
        },

        3: {
            HomeTerrain: TerrainYellow,
            EntityVizCode: 11,
            Starters: 20,
            Name: "Nomads",
            DamageBonus: 2,
            BaseEvasion: 0.0,
            DefenseBonus: 0,
        },

        4: {
            HomeTerrain: TerrainGreen,
            EntityVizCode: 12,
            Starters: 20,
            Name: "Sylvania",
            DamageBonus: 1,
            BaseEvasion: 0.12,
            DefenseBonus: 0, 
        },
    }
    
    // Paint base terrain
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            if x < 50 && y < 50 {
                // Top left
                w.Terrain[y*GridSize + x] = uint8(TerrainRed)
            } else if x > 50 && y < 50 {
                // Top right
                w.Terrain[y*GridSize + x] = uint8(TerrainBlue)
            } else if x < 50 && y > 50 {
                // Bottm left
                w.Terrain[y * GridSize + x] = uint8(TerrainYellow)
            } else if x > 50 && y > 50 {
                // Bottom right
                w.Terrain[y * GridSize + x] = uint8(TerrainGreen)
            } 
        }
    }

    // Center border
    for i := 0; i < GridSize; i++ {
        w.Terrain[i * GridSize + 50] = uint8(TerrainBorder)
        w.Terrain[50 * GridSize + i] = uint8(TerrainBorder)
    }

    homeFlats := make(map[TerrainType]bool)
    for _, cfg := range w.Tribes {
        homeFlats[cfg.HomeTerrain] = true
    }
    isHomeFlat := func(t TerrainType) bool {
        return homeFlats[t]
    }

    // === Trees (25 forests, 12-20 trees each) ===
    for i := 0; i < 25; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i-- // Retry
            continue
        }

        treesInCluster := 12 + rand.Intn(9)
        for j := 0; j < treesInCluster; j++ {
            nx, ny := cx + rand.Intn(15)-7, cy + rand.Intn(15)-7
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainTrees)
                    }
                }
            }
        }
    }

    // // === Rocks (15 clusters, 8-15 each, tighter) ===
    for i := 0; i < 15; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        rocksInCluster := 8 + rand.Intn(8)
        for j := 0; j < rocksInCluster; j++ {
            nx, ny := cx + rand.Intn(11)-5, cy + rand.Intn(11)-5
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainRocks)
                    }
                }
            }
        }
    }

    // // === Hills (20 areas, 20-40 each, larger) ===
    for i := 0; i < 20; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        hillsInArea := 20 + rand.Intn(21)
        for j := 0; j < hillsInArea; j++ {
            nx, ny := cx + rand.Intn(21)-10, cy + rand.Intn(21)-10
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainHills)
                    }
                }
            }
        }
    }

    for tribe, cfg := range w.Tribes {
        for i := 0; i < cfg.Starters; i++ {
            placed := false
            for attempts := 0; attempts < 1000; attempts++ {
                x, y := rand.Intn(GridSize), rand.Intn(GridSize)
                if w.Entities[y][x] == nil && TerrainType(w.Terrain[y*GridSize + x]) == cfg.HomeTerrain {
                    counter := w.nextEntityID[tribe]
                    if counter == nil {
                        counter = new(uint32)
                        w.nextEntityID[tribe] = counter
                    }
                    id := atomic.AddUint32(counter, 1)
                    w.Entities[y][x] = &Entity{
                        Health: 100,
                        Tribe: tribe,
                        Weapon: WeaponNone,
                        Armor: ArmorNone,
                        ID: id,
                        Evasion: cfg.BaseEvasion,
                        Rank: RankBase,
                    }
                    placed = true
                    break
                }
            }
            if !placed {
                log.Printf("Warning: Could not place starter for tribe %d (north-south map)", tribe)
            }
        }
    }
}

func InitNorthSouth(w *World) {
    for i := range w.Terrain {
        w.Terrain[i] = 0
    }

    w.Tribes = map[uint8]TribeConfig{
        1: {
            HomeTerrain: TerrainRed,
            EntityVizCode: 3,
            Starters: 20,
            Name: "Norsca",
            DamageBonus: 0, // No dmg bonus racial
            BaseEvasion: 0, // No evasion bonus racial
            DefenseBonus: 2, // Nords extra defense racial
        },

        2: {
            HomeTerrain: TerrainBlue,
            EntityVizCode: 5,
            Starters: 20,
            Name: "Sylvania",
            DamageBonus: 1, // Vampires racial (+1 dmg)
            BaseEvasion: 0.12, // Vampires racial (12% evasion)
            DefenseBonus: 0, // Vampires no racial defense
        },
    }

    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            if y < 50 {
                w.Terrain[y*GridSize + x] = uint8(TerrainRed)
            } else if y > 50 {
                w.Terrain[y*GridSize + x] = uint8(TerrainBlue)
            }
        }
    }

    for x := 0; x < GridSize; x++ {
        w.Terrain[50 * GridSize + x] = uint8(TerrainBorder)
    }

    homeFlats := make(map[TerrainType]bool)
    for _, cfg := range w.Tribes {
        homeFlats[cfg.HomeTerrain] = true
    }
    isHomeFlat := func(t TerrainType) bool {
        return homeFlats[t]
    }

    // === Trees (25 forests, 12-20 trees each) ===
    for i := 0; i < 25; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i-- // Retry
            continue
        }

        treesInCluster := 12 + rand.Intn(9)
        for j := 0; j < treesInCluster; j++ {
            nx, ny := cx + rand.Intn(15)-7, cy + rand.Intn(15)-7
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainTrees)
                    }
                }
            }
        }
    }

    // // === Rocks (15 clusters, 8-15 each, tighter) ===
    for i := 0; i < 15; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        rocksInCluster := 8 + rand.Intn(8)
        for j := 0; j < rocksInCluster; j++ {
            nx, ny := cx + rand.Intn(11)-5, cy + rand.Intn(11)-5
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainRocks)
                    }
                }
            }
        }
    }

    // // === Hills (20 areas, 20-40 each, larger) ===
    for i := 0; i < 20; i++ {
        cx, cy := rand.Intn(GridSize), rand.Intn(GridSize)
        idx := cy*GridSize + cx
        current := TerrainType(w.Terrain[idx])
        if w.Terrain[idx] == uint8(TerrainBorder) || !isHomeFlat(current) {
            i--
            continue
        }

        hillsInArea := 20 + rand.Intn(21)
        for j := 0; j < hillsInArea; j++ {
            nx, ny := cx + rand.Intn(21)-10, cy + rand.Intn(21)-10
            if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                nidx := ny*GridSize + nx
                if w.Terrain[nidx] != uint8(TerrainBorder) {
                    if isHomeFlat(TerrainType(w.Terrain[nidx])) {
                        w.Terrain[nidx] = uint8(TerrainHills)
                    }
                }
            }
        }
    }

    for tribe, cfg := range w.Tribes {
        for i := 0; i < cfg.Starters; i++ {
            placed := false
            for attempts := 0; attempts < 1000; attempts++ {
                x, y := rand.Intn(GridSize), rand.Intn(GridSize)
                if w.Entities[y][x] == nil && TerrainType(w.Terrain[y*GridSize + x]) == cfg.HomeTerrain {
                    counter := w.nextEntityID[tribe]
                    if counter == nil {
                        counter = new(uint32)
                        w.nextEntityID[tribe] = counter
                    }
                    id := atomic.AddUint32(counter, 1)
                    w.Entities[y][x] = &Entity{
                        Health: 100,
                        Tribe: tribe,
                        Weapon: WeaponNone,
                        Armor: ArmorNone,
                        ID: id,
                        Evasion: cfg.BaseEvasion,
                        Rank: RankBase,
                    }
                    placed = true
                    break
                }
            }
            if !placed {
                log.Printf("Warning: Could not place starter for tribe %d (north-south map)", tribe)
            }
        }
    }
}

func (w *World) InitCustomMap(terrain []uint8, assignments map[string]string) bool {
    w.Mu.Lock()
    defer w.Mu.Unlock()

    // Validate terrain size
    if len(terrain) != GridSize * GridSize {
        log.Println("Invalid custom map size")
        return false
    }

    copy(w.Terrain, terrain)

    // Predefined tribe stat templates
    type TribeTemplate struct {
        DamageBonus  int
        BaseEvasion  float64
        DefenseBonus int
        EntityVizCode uint8
    }
    
    tribeTemplates := map[string]TribeTemplate{
        "Wanderers": {DamageBonus: 0, BaseEvasion: 0.22, DefenseBonus: 0, EntityVizCode: 3},
        "Norsca":    {DamageBonus: 0, BaseEvasion: 0.0, DefenseBonus: 2, EntityVizCode: 5},
        "Nomads":    {DamageBonus: 2, BaseEvasion: 0.0, DefenseBonus: 0, EntityVizCode: 11},
        "Sylvania":  {DamageBonus: 1, BaseEvasion: 0.12, DefenseBonus: 0, EntityVizCode: 12},
    }

    w.Tribes = make(map[uint8]TribeConfig)
    tribeID := uint8(1)

    // Convert string terrain keys to uint8
    for terrainStr, tribeName := range assignments {
        if tribeName == "none" {
            continue
        }
        
        terrainType, err := strconv.Atoi(terrainStr)
        if err != nil {
            log.Printf("Invalid terrain type: %s", terrainStr)
            continue
        }
        
        template, ok := tribeTemplates[tribeName]
        if !ok {
            log.Printf("Unknown tribe name: %s", tribeName)
            continue
        }
        
        w.Tribes[tribeID] = TribeConfig{
            HomeTerrain:   TerrainType(terrainType),
            EntityVizCode: template.EntityVizCode,
            Starters:      20,
            Name:          tribeName,
            DamageBonus:   template.DamageBonus,
            BaseEvasion:   template.BaseEvasion,
            DefenseBonus:  template.DefenseBonus,
        }
        
        tribeID++
    }
    
    // Validate we have at least one tribe
    if len(w.Tribes) == 0 {
        log.Println("No tribes assigned in custom map")
        return false
    }
    
    // Place starter entities for each tribe
    for tribe, cfg := range w.Tribes {
        for i := 0; i < cfg.Starters; i++ {
            placed := false
            for attempts := 0; attempts < 1000; attempts++ {
                x, y := rand.Intn(GridSize), rand.Intn(GridSize)
                if w.Entities[y][x] == nil && TerrainType(w.Terrain[y*GridSize+x]) == cfg.HomeTerrain {
                    counter := w.nextEntityID[tribe]
                    if counter == nil {
                        counter = new(uint32)
                        w.nextEntityID[tribe] = counter
                    }
                    id := atomic.AddUint32(counter, 1)
                    w.Entities[y][x] = &Entity{
                        Health:  100,
                        Tribe:   tribe,
                        Weapon:  WeaponNone,
                        Armor:   ArmorNone,
                        ID:      id,
                        Evasion: cfg.BaseEvasion,
                        Rank:    RankBase,
                    }
                    placed = true
                    break
                }
            }
            if !placed {
                log.Printf("Warning: Could not place starter for tribe %d (%s)", tribe, cfg.Name)
            }
        }
    }
    
    log.Printf("Custom map initialized with %d tribes", len(w.Tribes))
    return true
}