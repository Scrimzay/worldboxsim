package world

import (
	"time"
)

type WeaponType uint8

const (
	WeaponNone WeaponType = 0
	WeaponWood WeaponType = 1
	WeaponStone WeaponType = 2
)

// depending on weapon type, adds bonus damage to entity
func (wt WeaponType) Bonus() int {
	switch wt {
	case WeaponWood:
		return 3

	case WeaponStone:
		return 4

	default:
		return 0
	}
}

type ArmorType uint8

const (
	ArmorNone ArmorType = 0
	ArmorWood ArmorType = 1
	ArmorStone ArmorType = 2
)

func (e ArmorType) Defensebonus() int {
	switch e {
	case ArmorWood:
		return 2

	case ArmorStone:
		return 3

	default:
		return 0
	}
}

// Calc damage for all implicits and explicits
func (e *Entity) TotalDamage(w *World) int {
	baseDamage := 5
	weaponBonus := e.Weapon.Bonus()
	rankBonus := e.Rank.DamageBonus()

	// Get racial passive damage bonus
	racialBonus := 0
	if cfg, ok := w.Tribes[e.Tribe]; ok {
		racialBonus = cfg.DamageBonus
	}

	return baseDamage + weaponBonus + racialBonus + rankBonus
}

// Calc total armor for all implicit and explicits
func (e *Entity) TotalArmor(w *World) int {
	armorBonus := e.Armor.Defensebonus()
	rankBonus := e.Rank.ArmorBonus()

	// Implement later for future races with def bonus
	// Get racial passive defense bonus
	racialBonus := 0
	if cfg, ok := w.Tribes[e.Tribe]; ok {
		racialBonus = cfg.DefenseBonus
	}

	return armorBonus + rankBonus + racialBonus
}

// handles clearing of tress/rocks by entity and tree regrowth (peace time only)
func HandleMiningAndRegrowth(w *World) {
	// w.mu.Lock()
	// defer w.mu.Unlock()
	
	currentTime := time.Now()
	regrowDuration := 20 * time.Second

	// Phase 1: Mining/Clearing (instant when entity is present on rock/tree)
	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			idx := y * GridSize + x
			ent := w.Entities[y][x]
			if ent != nil {
				terrain := TerrainType(w.Terrain[idx])
				if terrain == TerrainTrees || terrain == TerrainRocks {
					// Clear to the entity's tribe flat land
					cfg, ok := w.Tribes[ent.Tribe]
					if !ok {
						continue
					}

					res := w.resources[ent.Tribe]
					if res == nil {
						res = &TribeResources{}
						w.resources[ent.Tribe] = res
					}

					if terrain == TerrainTrees {
						res.Wood++
					} else if terrain == TerrainRocks {
						res.Stone++
					}

					w.Terrain[y * GridSize + x] = uint8(cfg.HomeTerrain)

					// Record clear time only for trees for regrowth
					if terrain == TerrainTrees {
						w.lastClearedTime[y][x] = currentTime
					}
				}
			}
		}
	}

	// Phase 2: Tree regrowth (only on cleared cells that have been empty)
	for y := 0; y < GridSize; y++ {
		for x := 0; x < GridSize; x++ {
			idx := y * GridSize + x
			if w.Entities[y][x] == nil { // Cell must be unoccupied
				lastClear := w.lastClearedTime[y][x]
				if !lastClear.IsZero() && currentTime.Sub(lastClear) >= regrowDuration {
					// Regrow only if cell is flat land
					currentTerrain := TerrainType(w.Terrain[idx])
					
					isHomeFlat := false
					for _, cfg := range w.Tribes {
						if currentTerrain == cfg.HomeTerrain {
							isHomeFlat = true
							break
						}
					}
					
					if isHomeFlat {
						w.Terrain[y * GridSize + x] = uint8(TerrainTrees)
						// Reset timer
						w.lastClearedTime[y][x] = time.Time{}
					}
				}
			}
		}
	}
}