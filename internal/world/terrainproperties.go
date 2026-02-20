package world

type TerrainType uint8

const (
	TerrainEmpty TerrainType = 0  // Bad/empty (no reprod, low preference)
    TerrainRed TerrainType = 1  // first tribe flat land (vertical and north v south)
    TerrainBlue TerrainType = 2  // second tribe flat land (vertical and north v south)
    TerrainBorder TerrainType = 4 // Neutral divider (crossable in war, avoided in peace)
    TerrainTrees TerrainType = 6  // Impassable neutral (forest)
    TerrainRocks TerrainType = 7  // Impassable neutral (mountains/rocks)
    TerrainHills TerrainType = 8  // Slow movement (higher cost, passable)
    TerrainYellow TerrainType = 9 // 4 quadrant map specific
	TerrainGreen TerrainType = 10 // 4 quadrant map specific
)

// Returns true if entities can move onto this terrain
func IsPassable(t TerrainType) bool {
	switch t {
	case TerrainEmpty, TerrainRed, TerrainBlue, TerrainBorder, TerrainHills, TerrainTrees, TerrainRocks, TerrainYellow, TerrainGreen:
		return true

	default:
		return false // Unknown/new types default to impassable for safety
	}
}

// returns true if this is a reproducible home terrain (1 or 2)
// used for reproduction and strong preference in peace
func IsFlatHomeTerrain(t TerrainType, tribe uint8) bool {
	if tribe == 1 {
		return t == TerrainRed
	}

	return t == TerrainBlue
}

// returns true if terrain belongs to the entity's tribe
func IsOwnTerrain(t TerrainType, w *World, tribe uint8) bool {
	if cfg, ok := w.Tribes[tribe]; ok {
		return t == cfg.HomeTerrain
	}

	return false
}

func IsEnemyTerrain(t TerrainType, w *World, tribe uint8) bool {
	_, myOk := w.Tribes[tribe]
	if !myOk {
		return false // Unknown tribe safety
	}

	for otherTribe, otherCfg := range w.Tribes {
		if otherTribe != tribe && t == otherCfg.HomeTerrain {
			return true
		}
	}

	return false
}

// returns a score addition for moving onto this terrain
// Positive = encouraged, negative = discouraged
// used in both peace and war movement scoring
func MoveScoreBonus(t TerrainType, w *World, myTribe uint8) float64 {
	switch t {
	case TerrainEmpty:
		return -2.0

	case TerrainBorder:
		return -3.0

	case TerrainHills:
		return -0.5
	}

	if IsOwnTerrain(t, w, myTribe) {
		return 2.0
	}

	if IsEnemyTerrain(t, w, myTribe) {
		return 8.0
	}

	return 0.0
}

func CanReproduceOn(t TerrainType, w *World, tribe uint8) bool {
	if cfg, ok := w.Tribes[tribe]; ok {
		return t == cfg.HomeTerrain
	}

	return false
}

func VictoryConquestColor(w *World, winnerTribe uint8) TerrainType {
	if cfg, ok := w.Tribes[winnerTribe]; ok {
		return cfg.HomeTerrain
	}

	return TerrainEmpty // Fallback that shouldnt hit
}