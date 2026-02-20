package world

import (
	"sync/atomic"
	"time"
)

type EntityStats struct {
	MoveChance float64 // Chance to attempt move
	ReproductionRate float64 // Chance to reproduce if space
    MaxDensityFraction float64 // Max fraction occupied before skipping reprod (0-1)
    ReprodCooldown time.Duration // Cooldown in seconds after reprod
}

type Rank uint8

const (
    RankBase Rank = 1 // Base, no mods
    RankSuper Rank = 2 // +1 dmg, +1 armor
    RankMega Rank = 3 // +2 dmg, +2 armor
)

func (r Rank) String() string {
    switch r {
    case RankBase:
        return "Base"

    case RankSuper:
        return "Super"

    case RankMega:
        return "Mega"

    default:
        return "Unknown"
    }
}

func (r Rank) DamageBonus() int {
    switch r {
    case RankSuper:
        return 1

    case RankMega:
        return 2

    default:
        return 0
    }
}

func (r Rank) ArmorBonus() int {
    switch r {
    case RankSuper:
        return 1

    case RankMega:
        return 2

    default:
        return 0
    }
}

func (r Rank) UpgradeCost() int64 {
    switch r {
    case RankSuper:
        return 50

    case RankMega:
        return 100

    default:
        return 0
    }
}

// this is for per-entity attributes
type Entity struct {
    Health int // 0-100
    Tribe uint8 // based on starting terrain, fixed at birth
    Weapon WeaponType
    Armor ArmorType
    ID uint32 // Unique sequential per tribe
    Evasion float64 // Chance to evade incoming attacks
    Rank Rank
}

type TribeResources struct {
    Wood int64
    Stone int64
}

func (w *World) PlaceEntity(x, y int, typ uint8) bool {
	w.Mu.Lock()
    defer w.Mu.Unlock()

    if x < 0 || x >= GridSize || y < 0 || y >= GridSize {
        return false // Out of bounds
    }

    if typ > 10 {
        return false // Invalid type
    }
    
    terrain := w.Terrain[y * GridSize + x]
    if typ == 0 {
        w.Terrain[y * GridSize + x] = 0
        w.Entities[y][x] = nil
        w.lastReprodTime[y][x] = time.Time{}

    } else if typ == 1 || typ == 2 || typ == 4 || typ == 9 || typ == 10 {
        w.Terrain[y * GridSize + x] = typ
        w.Entities[y][x] = nil // Remove any entity
        w.lastReprodTime[y][x] = time.Time{}

    } else if typ == 3 {
        terrainType := TerrainType(terrain)
		tribe, ok := w.GetTribeFromHomeTerrain(terrainType)
		if !ok || w.Entities[y][x] != nil {
			return false
		}

		counter := w.nextEntityID[tribe]
		if counter == nil {
			counter = new(uint32)
			w.nextEntityID[tribe] = counter
		}
		id := atomic.AddUint32(counter, 1)

        cfg := w.Tribes[tribe]
        evasion := cfg.BaseEvasion

		w.Entities[y][x] = &Entity{
			Health: 100,
			Tribe: tribe,
			Weapon: WeaponNone,
			Armor: ArmorNone,
			ID: id,
            Evasion: evasion,
            Rank: RankBase,
		}
		w.lastReprodTime[y][x] = time.Time{}
		return true

    } else if typ == 6 || typ == 7 || typ == 8 { // New neutral terrain
        w.Terrain[y*GridSize + x] = typ
        w.Entities[y][x] = nil
        w.lastReprodTime[y][x] = time.Time{}
        
        return true
    }

    return true
}

func (w *World) CountTerrain(typ uint8) int {
	w.Mu.RLock()
	defer w.Mu.RUnlock()

	count := 0
	for _, cell := range w.Terrain {
		if cell == typ {
			count++
		}
	}

	return count
}

func (w *World) CountEntitiesByTribe() map[uint8]int {
    w.Mu.RLock()
    defer w.Mu.RUnlock()

    counts := make(map[uint8]int)
	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			ent := w.Entities[y][x]
			if ent != nil {
				counts[ent.Tribe]++
			}
		}
	}

	return counts
}

func (w *World) GetEntity(x, y int) *Entity {
    if x < 0 || x >= GridSize || y < 0 || y >= GridSize {
        return nil
    }

    w.Mu.RLock()
    defer w.Mu.RUnlock()

    return w.Entities[y][x]
}

func (w *World) GetTribeResources(tribe uint8) (wood, stone int64) {
    w.Mu.RLock()
    defer w.Mu.RUnlock()
    if res, ok := w.resources[tribe]; ok {
        return res.Wood, res.Stone
    }

    return 0, 0
}