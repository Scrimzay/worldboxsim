package world

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	//"github.com/Scrimzay/worldboxsim/internal/world"
	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
}

type Broadcaster struct {
	world *World
	clients map[*websocket.Conn]bool
	register chan *websocket.Conn
	unregister chan *websocket.Conn
	updateTicker *time.Ticker // Dynamic for speed changes
	updateChan chan struct{} // Signal to reset ticker
	mu sync.RWMutex
	currentSpeed float64
	paused bool
	baseIntervalMs int64 // Base for 1x
	WriteMu map[*websocket.Conn]*sync.Mutex // Per=conn write locks
}

func NewBroadcaster(w *World) *Broadcaster {
	b := &Broadcaster{
		world: w,
		clients: make(map[*websocket.Conn]bool),
		register: make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		updateChan: make(chan struct{}, 1), // Buffered to avoid blocking
		currentSpeed: 1.0,
		paused: false,
		baseIntervalMs: 250,
		WriteMu: make(map[*websocket.Conn]*sync.Mutex),
	}

	b.resetUpdateTicker()
	return b
}

func (b *Broadcaster) resetUpdateTicker() {
	if b.updateTicker != nil {
		b.updateTicker.Stop()
	}

	intervalFloat := float64(b.baseIntervalMs) / b.currentSpeed
	interval := time.Duration(intervalFloat) * time.Millisecond
	if interval < 10 * time.Millisecond {
		interval = 10 * time.Millisecond // Min for high speeds (avoid overload)
	} else if interval > 10 * time.Second {
		interval = 10 * time.Second // Max for very low speeds
	}
	b.updateTicker = time.NewTicker(interval)
	log.Printf("Update ticker reset to %v (speed: %.2fx)", interval, b.currentSpeed)
}

func (b *Broadcaster) Run() {
	broadcastTicker := time.NewTicker(100 * time.Millisecond) // Grid broadcast ~10 fps
	defer func() {
		broadcastTicker.Stop()
		if b.updateTicker != nil {
			b.updateTicker.Stop()
		}
	}()

	for {
		select {
		case conn := <-b.register:
			b.clients[conn] = true
			b.mu.Lock() // Use mu for consistency
            b.WriteMu[conn] = &sync.Mutex{} // Init lock
            b.mu.Unlock()

            // Send initial world state
            grid := b.world.GetGridCopy()
            b.WriteMu[conn].Lock()
            if err := conn.WriteMessage(websocket.BinaryMessage, grid); err != nil {
                log.Println("Initial send error:", err)
                conn.Close()
                delete(b.clients, conn)
                delete(b.WriteMu, conn) // Cleanup
            }
            b.WriteMu[conn].Unlock()

			// Send initial state
			b.sendStatsTo(conn)

		case conn := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[conn]; ok {
				delete(b.clients, conn)
				delete(b.WriteMu, conn)
				conn.Close()
			}
			b.mu.Unlock()

		case <-broadcastTicker.C:
			grid := b.world.GetGridCopy()

			b.mu.RLock()
			for conn := range b.clients {
				if mu, ok := b.WriteMu[conn]; ok {
					mu.Lock()
					if err := conn.WriteMessage(websocket.BinaryMessage, grid); err != nil {
						log.Println("Broadcast error:", err)
						conn.Close()
						mu.Unlock()
						// Defer full cleanup to unregister channel
						b.unregister <- conn
						continue
					}
					mu.Unlock()
				}
			}
			b.mu.RUnlock()

		case <-b.updateTicker.C:
			b.mu.RLock()
			p := b.paused
			b.mu.RUnlock()
			if !p {
				b.world.Update() // Run sim tick
				b.BroadcastStats()
			}

		case <-b.updateChan:
			b.resetUpdateTicker()
		}
	}
}

func (b *Broadcaster) Register(conn *websocket.Conn) {
	b.register <- conn
}

func (b *Broadcaster) Unregister(conn *websocket.Conn) {
	b.unregister <- conn
}

// Set speed and reset ticker
func (b *Broadcaster) SetSpeed(speed float64) {
	b.mu.Lock()
	b.currentSpeed = speed
	b.mu.Unlock()
	
	select {
	case b.updateChan <- struct{}{}:

	default:
		// Already pending, skip
	}

	b.BroadcastStats()
}

// Toggle pause
func (b *Broadcaster) TogglePause() {
	b.mu.Lock()
	b.paused = !b.paused
	b.mu.Unlock()
	b.BroadcastStats()
}

// Send stats to a single client
func (b *Broadcaster) sendStatsTo(conn *websocket.Conn) {
	b.mu.RLock()
    speed := b.currentSpeed
    paused := b.paused
    b.mu.RUnlock()
   
    counts := b.world.CountEntitiesByTribe()
	
	tribeStats := make(map[string]map[string]interface{})

	b.world.Mu.RLock() // Need to read Tribes map
	for tribeID, cfg := range b.world.Tribes {
		strID := fmt.Sprintf("%d", tribeID)
		count := counts[tribeID]
		wood, stone := b.world.GetTribeResources(tribeID)

		tribeStats[strID] = map[string]interface{}{
			"count": count,
			"wood": wood,
			"stone": stone,
			"name": cfg.Name,
		}
	}
	b.world.Mu.RUnlock()

    stats := map[string]interface{}{
        "speed": speed,
        "paused": paused,
		"tribes": tribeStats,
    }

	if winner := b.world.GetWinner(); winner != "" {
		stats["winner"] = winner
	}

    data, err := json.Marshal(stats)
    if err != nil {
        log.Println("Stats marshal error:", err)
        return
    }

    b.WriteMu[conn].Lock()
    if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
        log.Println("Stats send error:", err)
    }
    b.WriteMu[conn].Unlock()
}

func (b *Broadcaster) BroadcastStats() {
    b.mu.RLock()
    speed := b.currentSpeed
    paused := b.paused
    b.mu.RUnlock()
    
    counts := b.world.CountEntitiesByTribe()
    
    tribeStats := make(map[string]map[string]interface{})

	b.world.Mu.RLock() // Need to read Tribes map
	for tribeID, cfg := range b.world.Tribes {
		strID := fmt.Sprintf("%d", tribeID)
		count := counts[tribeID]
		wood, stone := b.world.GetTribeResources(tribeID)

		tribeStats[strID] = map[string]interface{}{
			"count": count,
			"wood": wood,
			"stone": stone,
			"name": cfg.Name,
		}
	}
	b.world.Mu.RUnlock()

    for tribeID := range b.world.Tribes {
        strID := fmt.Sprintf("%d", tribeID)
        count := counts[tribeID]
        wood, stone := b.world.GetTribeResources(tribeID)
        tribeStats[strID] = map[string]interface{}{
            "count": count,
            "wood": wood,
            "stone": stone,
        }
    }

    stats := map[string]interface{}{
        "speed": speed,
        "paused": paused,
        "tribes": tribeStats,
    }
   
    if winner := b.world.GetWinner(); winner != "" {
        stats["winner"] = winner
    }

    data, err := json.Marshal(stats)
    if err != nil {
        log.Println("Stats marshal error:", err)
        return
    }

    b.mu.RLock()
    for conn := range b.clients {
        if mu, ok := b.WriteMu[conn]; ok {
            mu.Lock()
            if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
                log.Println("Stats broadcast error:", err)
                mu.Unlock()
                // Defer cleanup to unregister channel
                go func(c *websocket.Conn) {
                    b.unregister <- c
                }(conn)
                continue
            }
            mu.Unlock()
        }
    }
    b.mu.RUnlock()
}

func (b *Broadcaster) BroadcastGrid() {
    grid := b.world.GetGridCopy()
    
    b.mu.RLock()
    for conn := range b.clients {
        if mu, ok := b.WriteMu[conn]; ok {
            mu.Lock()
            if err := conn.WriteMessage(websocket.BinaryMessage, grid); err != nil {
                log.Println("Grid broadcast error:", err)
                mu.Unlock()
                go func(c *websocket.Conn) {
                    b.unregister <- c
                }(conn)
                continue
            }
            mu.Unlock()
        }
    }
    b.mu.RUnlock()
}