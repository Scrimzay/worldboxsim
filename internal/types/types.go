package types

// Unused, everything defined locally in packages, cant be fucked to make changes

// import (
// 	"sync"
// 	"time"

// 	"github.com/gorilla/websocket"
// )

// const GridSize = 100

// type World struct {
// 	Mu               sync.RWMutex
// 	Entities         [][]*Entity                   // [GridSize][GridSize]*Entity or nil
// 	Terrain          []uint8                       // Separate layer: 0 (empty/bad), 1 (red/left), 2 (blue/right), 4 (green/border)
// 	lastReprodTime   [GridSize][GridSize]time.Time // Per-call last reprod tick
// 	tickCount        int64                         // Global tick counter
// 	baseTickInterval time.Duration                 // For cooldown calc (set to 250ms)
// 	EntityStats      EntityStats
// 	warStarted       bool   // false initially, set to true on client action
// 	winner           string // "left", "right", "draw", etc..
// 	gameOver         bool
// 	conversionRate   float64                       // chance per tick to convert enemy terrain nder entity (war only)
// 	lastClearedTime  [GridSize][GridSize]time.Time // Tracks when a tree was last cleared
// 	resources        map[uint8]*TribeResources     // Key: tribe ID (1, 2, etc.)
// 	nextEntityID     map[uint8]*uint32             // Per-tribe sequential ID counter
// 	Tribes           map[uint8]TribeConfig         // Active tribes + config for this map
// }

// type TribeConfig struct {
// 	HomeTerrain   TerrainType // Primary flat for repro/starters/mining clear
// 	EntityVizCode uint8       // Render code for entities (3, 5, 9.. etc)
// 	Starters      int         // Starting entities per tribe
// 	Name          string
// 	DamageBonus   int     // Racial passive: bonus damage for related races
// 	BaseEvasion   float64 // Racial passive: bonus evasion for related races
// 	DefenseBonus  int     // Racial passive: bonus armor for related races
// }

// type EntityStats struct {
// 	MoveChance float64 // Chance to attempt move
// 	ReproductionRate float64 // Chance to reproduce if space
//     MaxDensityFraction float64 // Max fraction occupied before skipping reprod (0-1)
//     ReprodCooldown time.Duration // Cooldown in seconds after reprod
// }

// type Rank uint8

// type Entity struct {
//     Health int // 0-100
//     Tribe uint8 // based on starting terrain, fixed at birth
//     Weapon WeaponType
//     Armor ArmorType
//     ID uint32 // Unique sequential per tribe
//     Evasion float64 // Chance to evade incoming attacks
//     Rank Rank
// }

// type TribeResources struct {
//     Wood int64
//     Stone int64
// }

// type WeaponType uint8

// type ArmorType uint8

// type TerrainType uint8

// type Client struct {
// 	conn *websocket.Conn
// }

// type Broadcaster struct {
// 	world *World
// 	clients map[*websocket.Conn]bool
// 	register chan *websocket.Conn
// 	unregister chan *websocket.Conn
// 	updateTicker *time.Ticker // Dynamic for speed changes
// 	updateChan chan struct{} // Signal to reset ticker
// 	mu sync.RWMutex
// 	currentSpeed float64
// 	paused bool
// 	baseIntervalMs int64 // Base for 1x
// 	WriteMu map[*websocket.Conn]*sync.Mutex // Per=conn write locks
// }

// type PlaceAction struct {
// 	Action string `json:"action"`
// 	X int `json:"x"`
// 	Y int `json:"y"`
// 	Type uint8 `json:"type"`
// }

// type InspectAction struct {
// 	Action string `json:"action"`
// 	X int `json:"x"`
// 	Y int `json:"y"`
// }

// type PlaceBatchAction struct {
// 	Action string `json:"action"`
// 	Places []struct {
// 		X int `json:"x"`
// 		Y int `json:"y"`
// 		Type uint8 `json:"type"`
// 	} `json:"places"`
// }

// type SpeedAction struct {
// 	Action string `json:"action"`
// 	Multiplier float64 `json:"multiplier"`
// }

// type PauseAction struct {
// 	Action string `json:"action"`
// }