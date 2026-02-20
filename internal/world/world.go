package world

import (
	//"math"
	//"log"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

const GridSize = 100

type World struct {
	Mu sync.RWMutex
	Entities [][]*Entity // [GridSize][GridSize]*Entity or nil
    Terrain []uint8 // Separate layer: 0 (empty/bad), 1 (red/left), 2 (blue/right), 4 (green/border)
    lastReprodTime [GridSize][GridSize]time.Time // Per-call last reprod tick
    tickCount int64 // Global tick counter
    baseTickInterval time.Duration // For cooldown calc (set to 250ms)
	EntityStats EntityStats
    warStarted bool // false initially, set to true on client action
    winner string // "left", "right", "draw", etc..
    gameOver bool
    conversionRate float64 // chance per tick to convert enemy terrain nder entity (war only)
    lastClearedTime [GridSize][GridSize]time.Time // Tracks when a tree was last cleared
    resources map[uint8]*TribeResources // Key: tribe ID (1, 2, etc.)
    nextEntityID map[uint8]*uint32 // Per-tribe sequential ID counter
    Tribes map[uint8]TribeConfig // Active tribes + config for this map
}

type TribeConfig struct {
    HomeTerrain TerrainType // Primary flat for repro/starters/mining clear
    EntityVizCode uint8 // Render code for entities (3, 5, 9.. etc)
    Starters int // Starting entities per tribe
    Name string
    DamageBonus int // Racial passive: bonus damage for related races
    BaseEvasion float64 // Racial passive: bonus evasion for related races
    DefenseBonus int // Racial passive: bonus armor for related races
}

func New() *World {
	rand.Seed(time.Now().UnixNano())
	w := &World{
		Entities: make([][]*Entity, GridSize),
        Terrain: make([]uint8, GridSize * GridSize),
        lastReprodTime: [GridSize][GridSize]time.Time{},
        baseTickInterval: 250 * time.Millisecond,
        warStarted: false,
        conversionRate: 0.20, // 20% chance per tick
		EntityStats: EntityStats{
			MoveChance: 0.2, // 20% move chance
			ReproductionRate: 0.005, // 0.5% reproduction chance
            MaxDensityFraction: 0.030, // should be 4% but its 40% for some reason so dont go above 0.1%
            ReprodCooldown: 1 * time.Minute, // 5 minutes
		},
	}

    for i := range w.Entities {
        w.Entities[i] = make([]*Entity, GridSize)
    }

    w.lastClearedTime = [GridSize][GridSize]time.Time{}
    w.resources = make(map[uint8]*TribeResources)
    w.nextEntityID = make(map[uint8]*uint32) // Start at 1 for each tribe

    // Default to classic map
	//w.InitMap("northsouth")

	return w
}

// Color guide for world: 1 = red, 2 = blue, 3 = yellow, 4 = green

func (w *World) InitMap(mapName string) {
    w.Mu.Lock()
    defer w.Mu.Unlock()

    switch mapName {
    case "vertical":
        InitVerticalSplit(w)

    case "fourquadrants":
        InitFourQuadrants(w)

    case "northsouth":
        InitNorthSouth(w)

    case "custommap":
        for i := range w.Terrain {
            w.Terrain[i] = 0
        }
        w.Tribes = make(map[uint8]TribeConfig)
        log.Println("Custom map made")

    // case "islands":

    default:
        log.Printf("Unknow map '%s', falling back to vertical", mapName)
        InitVerticalSplit(w)
    }
}

// Safe read for broadcasting
func (w *World) GetGridCopy() []uint8 {
	w.Mu.RLock()
	defer w.Mu.RUnlock()

	copyGrid := make([]uint8, GridSize * GridSize)
	for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
           ent := w.Entities[y][x]
           if ent != nil {
                if cfg, ok := w.Tribes[ent.Tribe]; ok {
                    copyGrid[y * GridSize + x] = cfg.EntityVizCode
                } else {
                    copyGrid[y * GridSize + x] = 3 // Fallback unknown
                }
            } else {
                copyGrid[y * GridSize + x] = w.Terrain[y * GridSize + x]
            }
        }
    }

	return copyGrid
}

func (w *World) GetTribeFromHomeTerrain(t TerrainType) (uint8, bool) {
    for tribe, cfg := range w.Tribes {
        if cfg.HomeTerrain == t {
            return tribe, true
        }
    }

    return 0, false
}

func (w *World) StartWar() {
    w.Mu.Lock()
    defer w.Mu.Unlock()
    w.warStarted = true
}

func (w *World) GetWinner() string {
    w.Mu.RLock()
    defer w.Mu.RUnlock()
    return w.winner
}

func (w *World) IsGameOver() bool {
    w.Mu.RLock()
    defer w.Mu.RUnlock()
    return w.gameOver
}

func (w *World) Reset() {
    w.Mu.Lock()
    defer w.Mu.Unlock()
    
    // Clear entities and cooldowns
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            w.Entities[y][x] = nil
            w.lastReprodTime[y][x] = time.Time{}
            w.lastClearedTime[y][x] = time.Time{}
        }
    }

    // reset resources
    w.resources = make(map[uint8]*TribeResources)
    w.nextEntityID = make(map[uint8]*uint32)

    // reset state
    w.warStarted = false
    w.gameOver = false
    w.winner = ""
    
    // Restore initial terrain and starting entities
    //w.InitMap("northsouth")
}

func (w *World) PreWarUpdate() {
    w.Mu.Lock()
    defer w.Mu.Unlock()

    defer func() {
        if r := recover(); r != nil {
            log.Printf("PANIC in PreWarUpdate: %v\nStack trace:\n%s", r, debug.Stack())
        }
    }()

    newEntities := make([][]*Entity, GridSize)
    for i := range newEntities {
        newEntities[i] = make([]*Entity, GridSize)
    }
    directions := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} // Up, down, left, right
   
    // Phase 1: Collect potential moves
    type PotentialMove struct {
        fromX, fromY int
        toX, toY     int
    }
    potentialMoves := []PotentialMove{}

    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := w.Entities[y][x]
            if ent != nil && rand.Float64() < w.EntityStats.MoveChance {
                myTribe := ent.Tribe

                // Mining impulse
                resourceDirs := []int{}
                for d, dir := range directions {
                    nx, ny := x + dir[0], y + dir[1]
                    if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                        targetTerrain := TerrainType(w.Terrain[ny*GridSize + nx])
                        if w.Entities[ny][nx] == nil && (targetTerrain == TerrainTrees || targetTerrain == TerrainRocks) {
                            resourceDirs = append(resourceDirs, d)
                        }
                    }
                }

                if len(resourceDirs) > 0 && rand.Float64() < 0.20 {
                    d := resourceDirs[rand.Intn(len(resourceDirs))]
                    dir := directions[d]
                    nx, ny := x + dir[0], y + dir[1]
                    potentialMoves = append(potentialMoves, PotentialMove{fromX: x, fromY: y, toX: nx, toY: ny})
                    continue
                }

                // Normal scoring if not mining
                bestScore := -1.0
                bestDirs := []int{}
                for d, dir := range directions {
                    nx, ny := x + dir[0], y + dir[1]
                    if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                        targetTerrain := TerrainType(w.Terrain[ny*GridSize + nx])
                        if w.Entities[ny][nx] == nil && IsPassable(targetTerrain) {
                            score := MoveScoreBonus(targetTerrain, w, myTribe)
                            if score > bestScore {
                                bestScore = score
                                bestDirs = []int{d}
                            } else if score == bestScore {
                                bestDirs = append(bestDirs, d)
                            }
                        }
                    }
                }

                if bestScore > 0 && len(bestDirs) > 0 {
                    d := bestDirs[rand.Intn(len(bestDirs))]
                    dir := directions[d]
                    nx, ny := x + dir[0], y + dir[1]
                    potentialMoves = append(potentialMoves, PotentialMove{fromX: x, fromY: y, toX: nx, toY: ny})
                }
            }
        }
    }

    // Initially set all entities to stay
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            newEntities[y][x] = w.Entities[y][x]
        }
    }

    // Phase 2: Resolve move conflicts (move cooldowns with entities; terrain preserved)
    type TargetKey struct { tx, ty int }
    targetMovers := make(map[TargetKey][]PotentialMove)

    for _, pm := range potentialMoves {
        key := TargetKey{tx: pm.toX, ty: pm.toY}
        targetMovers[key] = append(targetMovers[key], pm)
    }

    for _, movers := range targetMovers {
        if len(movers) > 0 {
            // Random winner
            winner := movers[rand.Intn(len(movers))]
            // Apply move
            newEntities[winner.toY][winner.toX] = w.Entities[winner.fromY][winner.fromX]
            newEntities[winner.fromY][winner.fromX] = nil // Terrain stays
           
            // Move cooldown to new position
            w.lastReprodTime[winner.toY][winner.toX] = w.lastReprodTime[winner.fromY][winner.fromX]
           
            // Reset old position
            w.lastReprodTime[winner.fromY][winner.fromX] = time.Time{}
            // Losers stay (already set, including their cooldowns)
        }
    }

    // Phase 3: Reproduction
    currentTime := time.Now()
    cooldownDur := w.EntityStats.ReprodCooldown

    // Count per tribe
    tribeCounts := make(map[uint8]int)
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x]
            if ent != nil {
                tribeCounts[ent.Tribe]++
            }
        }
    }

    // Skip repro per tribe if over global density fraction
    skipReprod := make(map[uint8]bool)
    totalCells := GridSize * GridSize
    for tribe, count := range tribeCounts {
        density := float64(count) / float64(totalCells)
        skipReprod[tribe] = density > w.EntityStats.MaxDensityFraction
    }

    // Collect spawns without applying yet
    type Spawn struct {
        nx, ny int
        tribe uint8
    }
    spawns := []Spawn{}
    
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x]
            if ent != nil {
                if skipReprod[ent.Tribe] {
                    continue
                }

                last := w.lastReprodTime[y][x]
                if !last.IsZero() && currentTime.Sub(last) < cooldownDur {
                    continue // Cooldown active
                }
               
                if rand.Float64() < w.EntityStats.ReproductionRate {
                    rand.Shuffle(len(directions), func(i, j int) { directions[i], directions[j] = directions[j], directions[i]})
                    for _, dir := range directions {
                        nx, ny := x + dir[0], y + dir[1]
                        if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                            targetTerrain := TerrainType(w.Terrain[ny * GridSize + nx])
                            if newEntities[ny][nx] == nil && CanReproduceOn(targetTerrain, w, ent.Tribe) {
                                spawns = append(spawns, Spawn{nx: nx, ny: ny, tribe: ent.Tribe})
                                w.lastReprodTime[y][x] = currentTime // Set parent cooldown
                                // Approx density update
                                tribeCounts[ent.Tribe]++
                                density := float64(tribeCounts[ent.Tribe]) / float64(totalCells)
                                if density > w.EntityStats.MaxDensityFraction {
                                    skipReprod[ent.Tribe] = true
                                }
                                break
                            }
                        }
                    }
                }
            }
        }
    }

    // Phase 4: Arming and crafting
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            idx := w.Terrain[y * GridSize + x]
            ent := newEntities[y][x]
            if ent != nil {
                cfg, ok := w.Tribes[ent.Tribe]
                if !ok {
                    continue
                }

                terrain := TerrainType(idx)
                if terrain == cfg.HomeTerrain && rand.Float64() < 0.02 {
                    res := w.resources[ent.Tribe]
                    if res == nil {
                        continue
                    }

                    if ent.Weapon == WeaponNone && res.Wood >= 3 {
                        res.Wood -= 3
                        ent.Weapon = WeaponWood
                    } else if ent.Weapon == WeaponWood && res.Stone >= 3 {
                        res.Stone -= 3
                        ent.Weapon = WeaponStone
                    }

                    if ent.Armor == ArmorNone && res.Wood >= 5 {
                        res.Wood -= 5
                        ent.Armor = ArmorWood
                    } else if ent.Armor == ArmorWood && res.Stone >= 5 {
                        res.Stone -= 5
                        ent.Armor = ArmorStone
                    }

                    if ent.Rank == RankBase && res.Wood >= RankSuper.UpgradeCost() {
                        res.Wood -= RankSuper.UpgradeCost()
                        ent.Rank = RankSuper    
                    } else if ent.Rank == RankSuper && res.Wood >= RankMega.UpgradeCost() {
                        res.Wood -= RankMega.UpgradeCost()
                        ent.Rank = RankMega
                    }
                }
            }
        }
    }

    // Apply all spawns and set child cooldowns
    for _, s := range spawns {
        counter := w.nextEntityID[s.tribe]
        if counter == nil {
            counter = new(uint32)
            w.nextEntityID[s.tribe] = counter
        }
        id := atomic.AddUint32(counter, 1)

        cfg := w.Tribes[s.tribe]
        newEntities[s.ny][s.nx] = &Entity{
            Health: 100,
            Tribe: s.tribe,
            Weapon: WeaponNone,
            Armor: ArmorNone,
            ID: id,
            Evasion: cfg.BaseEvasion,
            Rank: RankBase,
        }
        w.lastReprodTime[s.ny][s.nx] = currentTime // Set child cooldown to match
    }

    w.Entities = newEntities
    HandleMiningAndRegrowth(w)
}

// SiMulation update tick
func (w *World) Update() {
    w.Mu.RLock()
    started := w.warStarted
    w.Mu.RUnlock()
    if !started {
        w.PreWarUpdate()
        return
    }

    defer func() {
        if r := recover(); r != nil {
            log.Printf("PANIC in Update (war): %v\nStack trace:\n%s", r, debug.Stack())
            w.gameOver = true // Optional: freeze sim gracefully
        }
    }()
   
    // post-war logic
    w.Mu.Lock()
    defer w.Mu.Unlock()

    // Early exit if game is already over — freezes the world after victory
    if w.gameOver {
        return
    }

    newEntities := make([][]*Entity, GridSize)
    for i := range newEntities {
        newEntities[i] = make([]*Entity, GridSize)
    }
    directions := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} // Up, down, left, right
   
    // Pre-compute enemy centers per tribe
    type TribeCenter struct {
        XSum, YSum float64
        Count int
    }
    tribeCenters := make(map[uint8]TribeCenter)
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := w.Entities[y][x]
            if ent != nil {
                tc := tribeCenters[ent.Tribe]
                tc.XSum += float64(x)
                tc.YSum += float64(y)
                tc.Count++
                tribeCenters[ent.Tribe] = tc
            }
        }
    }

    // Compute actual centers
    centers := make(map[uint]struct{ CX, CY float64 })
    for tribe, tc := range tribeCenters {
        if tc.Count > 0 {
            centers[uint(tribe)] = struct{ CX, CY float64 }{
                CX: tc.XSum / float64(tc.Count),
                CY: tc.YSum / float64(tc.Count),
            }
        }
    }
   
    // Phase 1: Collect potential moves
    type PotentialMove struct {
        fromX, fromY int
        toX, toY     int
    }
    potentialMoves := []PotentialMove{}

    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := w.Entities[y][x]
            if ent == nil || rand.Float64() >= w.EntityStats.MoveChance {
                continue
            }

            myTribe := ent.Tribe
            _, myOk := w.Tribes[myTribe]
            if !myOk {
                continue // Unknown tribe safety
            }

            // Compute enemy center: average of all other tribes' centers
            var enemyXSum, enemyYSum float64
            var enemyTotalCount int
            hasEnemies := false
            for otherTribe, otherCenter := range centers {
                if otherTribe != uint(myTribe) {
                    otherTC := tribeCenters[uint8(otherTribe)]
                    enemyXSum += otherCenter.CX * float64(otherTC.Count)
                    enemyYSum += otherCenter.CY * float64(otherTC.Count)
                    enemyTotalCount += otherTC.Count
                    hasEnemies = true
                }
            }

            var enemyCX, enemyCY float64
            if hasEnemies && enemyTotalCount > 0 {
                enemyCX = enemyXSum / float64(enemyTotalCount)
                enemyCY = enemyYSum / float64(enemyTotalCount)
            }

            currentDist := 0.0
            if hasEnemies {
                currentDist = math.Abs(float64(x) - enemyCX) + math.Abs(float64(y) - enemyCY)
            }

            bestScore := -1.0
            bestDirs := []int{}

            for d, dir := range directions {
                nx, ny := x + dir[0], y + dir[1]
                if nx < 0 || nx >= GridSize || ny < 0 || ny >= GridSize {
                    continue
                }

                targetTerrain := TerrainType(w.Terrain[ny * GridSize + nx])
                if w.Entities[ny][nx] != nil || !IsPassable(targetTerrain) {
                    continue
                }

                score := MoveScoreBonus(targetTerrain, w, myTribe)

                // Stron invasion bonus for stepping on enemy flat
                if IsEnemyTerrain(targetTerrain, w, myTribe) {
                    score += 12.0
                }

                // Local aggression
                localEnemies := 0
                for edy := -1; edy <= 1; edy++ {
                    for edx := -1; edx <= 1; edx++ {
                        ex, ey := nx + edx, ny + edy
                        if ex >= 0 && ex < GridSize && ey >= 0 && ey < GridSize {
                            enemyEnt := w.Entities[ey][ex]
                            if enemyEnt != nil && enemyEnt.Tribe != myTribe {
                                localEnemies++
                            }
                        }
                    }
                }

                score += float64(localEnemies) * 4.0

                // Frontier bonus
                frontierBonus := 0
                for edy := -1; edy <= 1; edy++ {
                    for edx := -1; edx <= 1; edx++ {
                        ex, ey := nx + edx, ny + edy
                        if ex >= 0 && ex < GridSize && ey >= 0 && ey < GridSize {
                            if IsEnemyTerrain(TerrainType(w.Terrain[ey*GridSize + ex]), w, myTribe) {
                                frontierBonus++
                            }
                        }
                    }
                }
                score += float64(frontierBonus) * 3.0

                // Global pull (weakened, only if closer)
                if hasEnemies {
                    newDist := math.Abs(float64(nx)-enemyCX) + math.Abs(float64(ny)-enemyCY)
                    reduction := currentDist - newDist
                    if reduction > 0 {
                        score += reduction * 3.0
                    }
                }

                if score > bestScore {
                    bestScore = score
                    bestDirs = []int{d}
                } else if score == bestScore {
                    bestDirs = append(bestDirs, d)
                }
            }

            if bestScore > 0 && len(bestDirs) > 0 {
                d := bestDirs[rand.Intn(len(bestDirs))]
                dir := directions[d]
                nx, ny := x + dir[0], y + dir[1]
                potentialMoves = append(potentialMoves, PotentialMove{fromX: x, fromY: y, toX: nx, toY: ny})
            }
        }
    }

    // Initially set all entities to stay
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            newEntities[y][x] = w.Entities[y][x]
        }
    }

    // Phase 2: Resolve move conflicts
    type TargetKey struct { tx, ty int }
    targetMovers := make(map[TargetKey][]PotentialMove)
    
    for _, pm := range potentialMoves {
        key := TargetKey{tx: pm.toX, ty: pm.toY}
        targetMovers[key] = append(targetMovers[key], pm)
    }

    for _, movers := range targetMovers {
        if len(movers) > 0 {
            winner := movers[rand.Intn(len(movers))]
            newEntities[winner.toY][winner.toX] = w.Entities[winner.fromY][winner.fromX]
            newEntities[winner.fromY][winner.fromX] = nil
            w.lastReprodTime[winner.toY][winner.toX] = w.lastReprodTime[winner.fromY][winner.fromX]
            w.lastReprodTime[winner.fromY][winner.fromX] = time.Time{}
        }
    }

    // Phase 2.5: Fighting
    damageAccum := make([][]int, GridSize)
    for i := range damageAccum {
        damageAccum[i] = make([]int, GridSize)
    }

    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x]
            if ent != nil {
                // Method that inclues racial damage
                myDmg := ent.TotalDamage(w)

                for _, dir := range directions {
                    nx, ny := x + dir[0], y + dir[1]
                    if nx >= 0 && nx < GridSize && ny >= 0 && ny < GridSize {
                        neighbor := newEntities[ny][nx]
                        if neighbor != nil && neighbor.Tribe != ent.Tribe {
                            // Evasion check for hit or not
                            if rand.Float64() >= neighbor.Evasion {
                                damageAccum[ny][nx] += myDmg
                            }
                        }
                    }
                }
            }
        }
    }

    // Apply damage and deaths
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x]
            if ent != nil {
                incoming := damageAccum[y][x]
                defense := ent.TotalArmor(w)
                effectiveDmg := incoming - defense
                if effectiveDmg < 1 /*&& incoming > 0*/ {
                    effectiveDmg = 1 // never below 0 to prevent unkillables
                }
                ent.Health -= effectiveDmg
                if ent.Health <= 0 {
                    newEntities[y][x] = nil
                    w.lastReprodTime[y][x] = time.Time{}
                }
            }
        }
    }

    // Phase 3: Terrain conversion (war only — only flat enemy land)
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x]
            if ent != nil {
                terrain := TerrainType(w.Terrain[y*GridSize + x])
                if IsEnemyTerrain(terrain, w, ent.Tribe) && rand.Float64() < w.conversionRate {
                    cfg, ok := w.Tribes[ent.Tribe]
                    if ok {
                        w.Terrain[y * GridSize + x] = uint8(cfg.HomeTerrain)
                    }
                }
            }
        }
    }

    // Attrition / regen on harsh terrain
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := newEntities[y][x] // Use final w.Entities after moves/combat/conversion
            if ent != nil {
                terrain := TerrainType(w.Terrain[y*GridSize + x])
                if terrain == TerrainHills || terrain == TerrainRocks || terrain == TerrainTrees {
                    // Attrition: lose health on hills (harsh terrain tires troops)
                    ent.Health -= 3 // Adjust this value — 3 feels noticeable but not instant death
                    if ent.Health <= 0 {
                        ent.Health = 0
                        newEntities[y][x] = nil
                        w.lastReprodTime[y][x] = time.Time{}
                    }
                } else {
                    // Regen when off hills (back to full strength)
                    if ent.Health < 100 {
                        ent.Health += 3 // Faster regen off hills — quickly back to ~100
                        if ent.Health > 100 {
                            ent.Health = 100
                        }
                    }
                }
            }
        }
    }

    w.Entities = newEntities

    // Victory Detection + Full Terrain Conquest (leaves border + natural features)
    aliveCounts := make(map[uint8]int)
    for y := 0; y < GridSize; y++ {
        for x := 0; x < GridSize; x++ {
            ent := w.Entities[y][x]
            if ent != nil {
                aliveCounts[ent.Tribe]++
            }
        }
    }

    aliveTribes := 0
    var winnerTribe uint8
    for tribe, count := range aliveCounts {
        if count > 0 {
            aliveTribes++
            winnerTribe = tribe
        }
    }

    if aliveTribes == 0 {
        w.winner = "draw"
        w.gameOver = true
    } else if aliveTribes == 1 {
        w.winner = fmt.Sprintf("%d", winnerTribe)
        w.gameOver = true

        winnerCfg, ok := w.Tribes[winnerTribe]
        if ok {
            winnerHome := winnerCfg.HomeTerrain
            for i := range w.Terrain {
                t := TerrainType(w.Terrain[i])
                if IsEnemyTerrain(t, w, winnerTribe) {
                    w.Terrain[i] = uint8(winnerHome)
                }
            }
        }
    }
}