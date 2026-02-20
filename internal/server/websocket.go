package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Scrimzay/worldboxsim/internal/world"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {return true},
}

type PlaceAction struct {
	Action string `json:"action"`
	X int `json:"x"`
	Y int `json:"y"`
	Type uint8 `json:"type"`
}

type InspectAction struct {
	Action string `json:"action"`
	X int `json:"x"`
	Y int `json:"y"`
}

type PlaceBatchAction struct {
	Action string `json:"action"`
	Places []struct {
		X int `json:"x"`
		Y int `json:"y"`
		Type uint8 `json:"type"`
	} `json:"places"`
}

type SpeedAction struct {
	Action string `json:"action"`
	Multiplier float64 `json:"multiplier"`
}

type PauseAction struct {
	Action string `json:"action"`
}

type CustomMapAction struct {
	Action string `json:"action"`
	Terrain []uint8 `json:"terrain"`
	TribeAssignments map[string]string `json:"tribeAssignments"`
}

func HandleWebsocket(broadcaster *world.Broadcaster, gameWorld *world.World) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WS upgrade error:", err)
			return
		}

		broadcaster.Register(conn)

		for {
			msgType, msg, err := conn.ReadMessage()
			if err != nil {
				broadcaster.Unregister(conn)
				break
			}

			if msgType == websocket.TextMessage {
				var baseAction map[string]interface{}
				if err := json.Unmarshal(msg, &baseAction); err != nil {
					log.Println("JSON parse error:", err)
					continue
				}

				action, ok := baseAction["action"].(string)
				if !ok {
					continue
				}

				switch action {
				case "place":
					var place PlaceAction
					json.Unmarshal(msg, &place)
					if gameWorld.PlaceEntity(place.X, place.Y, place.Type) {
						fmt.Println("placed entity 1")
						broadcaster.BroadcastStats()
					}

				case "set_speed":
					var speed SpeedAction
					json.Unmarshal(msg, &speed)
					if speed.Multiplier > 0 {
						broadcaster.SetSpeed(speed.Multiplier)
					}

				case "place_batch":
					var batch PlaceBatchAction
					json.Unmarshal(msg, &batch)
					placed := false
					for _, p := range batch.Places {
						if gameWorld.PlaceEntity(p.X, p.Y, p.Type) {
							placed = true
							fmt.Println("placed entity 2")
						}
					}
					if placed {
						broadcaster.BroadcastGrid()
						broadcaster.BroadcastStats()
					}

				case "start_war":
					gameWorld.ConvertBordersToTerrain()
					gameWorld.StartWar()
					broadcaster.BroadcastGrid()
					broadcaster.BroadcastStats()

				case "reset":
					gameWorld.Reset()
					broadcaster.BroadcastGrid()
					broadcaster.BroadcastStats()

				case "inspect":
					var inspect InspectAction
					if err := json.Unmarshal(msg, &inspect); err != nil {
						log.Println("Inspect marshal error:", err)
						continue
					}

					// Bounds check
					if inspect.X < 0 || inspect.X >= world.GridSize || inspect.Y < 0 || inspect.Y >= world.GridSize {
						continue
					}

					ent := gameWorld.GetEntity(inspect.X, inspect.Y)
					
					resp := map[string]interface{}{
						"action": "inspect_response",
						"empty": ent == nil,
					}

					if ent != nil {
						tribeName := fmt.Sprintf("Tribe %d", ent.Tribe)
						
						gameWorld.Mu.RLock()
						cfg, ok := gameWorld.Tribes[ent.Tribe]
						if ok {
							tribeName = cfg.Name
							log.Printf("DEBUG: Tribe %d (%s) - DamageBonus: %d, DefenseBonus: %d, Evasion: %.2f", 
        								ent.Tribe, cfg.Name, cfg.DamageBonus, cfg.DefenseBonus, cfg.BaseEvasion)
						}
						gameWorld.Mu.RUnlock()

						weaponStr := "None"
						damageBonus := 0
						if ent.Weapon == world.WeaponWood {
							weaponStr = "Wood Sword"
							damageBonus = 3
						} else if ent.Weapon == world.WeaponStone {
							weaponStr = "Stone Sword"
							damageBonus = 4
						}

						armorStr := "None"
						defenseBonus := 0
						if ent.Armor == world.ArmorWood {
							armorStr = "Wood Armor"
							defenseBonus = 2
						} else if ent.Armor == world.ArmorStone {
							armorStr = "Stone Armor"
							defenseBonus = 3
						}

						// Calculate total damage including racial passive + rank
						racialDamageBonus := 0
						racialDefenseBonus := 0
						if ok {
							racialDamageBonus = cfg.DamageBonus
							racialDefenseBonus = cfg.DefenseBonus
						}
						rankDamageBonus := ent.Rank.DamageBonus()
						rankArmorBonus := ent.Rank.ArmorBonus()
						totalDamage := 5 + damageBonus + racialDamageBonus + rankDamageBonus
						totalDefense := defenseBonus + rankArmorBonus + racialDefenseBonus

						// Format evasion as percentage
						evasionPercent := int(ent.Evasion * 100)

						resp["name"] = fmt.Sprintf("%s Entity #%d", tribeName, ent.ID)
						resp["health"] = ent.Health
						resp["weapon"] = weaponStr
						resp["armor"] = armorStr
						resp["damage"] = totalDamage
						resp["defense"] = totalDefense
						resp["evasion"] = evasionPercent
						resp["racialDamage"] = racialDamageBonus
						resp["racialDefense"] = racialDefenseBonus
						resp["rank"] = ent.Rank.String()
						resp["rankDamage"] = rankDamageBonus
						resp["rankArmor"] = rankArmorBonus
					
						log.Printf("DEBUG: Sending inspect - racialDefense: %d, totalDefense: %d, defenseBonus: %d, rankArmorBonus: %d", 
    					racialDefenseBonus, totalDefense, defenseBonus, rankArmorBonus)
					}

					data, err := json.Marshal(resp)
					if err != nil {
						log.Println("Inspect response marshal error:", err)
						continue
					}

					// Use the per-connection write lock
					if mu, ok := broadcaster.WriteMu[conn]; ok {
						mu.Lock()
						if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
							log.Println("Inspect send error:", err)
							mu.Unlock()
							broadcaster.Unregister(conn)
							break
						}
						mu.Unlock()
					}
				
				case "init_custom_map":
					var customMap CustomMapAction
					json.Unmarshal(msg, &customMap)

					success := gameWorld.InitCustomMap(customMap.Terrain, customMap.TribeAssignments)
					
					if success {
						broadcaster.BroadcastGrid()
						broadcaster.BroadcastStats()
						
						// Build tribe info to send to client
						gameWorld.Mu.RLock()
						tribeInfo := make(map[string]interface{})
						for tribeID, cfg := range gameWorld.Tribes {
							tribeInfo[fmt.Sprintf("%d", tribeID)] = map[string]interface{}{
								"name":          cfg.Name,
								"entityVizCode": cfg.EntityVizCode,
								"homeTerrain":   cfg.HomeTerrain,
							}
						}
						gameWorld.Mu.RUnlock()
						
						// Send confirmation with tribe data
						confirmMsg := map[string]interface{}{
							"action": "custom_map_initialized",
							"tribes": tribeInfo,
						}
						confirmData, _ := json.Marshal(confirmMsg)
						
						if mu, ok := broadcaster.WriteMu[conn]; ok {
							mu.Lock()
							conn.WriteMessage(websocket.TextMessage, confirmData)
							mu.Unlock()
						}
					} else {
						// Send error back
						errMsg := map[string]string{"action": "custom_map_error", "error": "Failed to initialize map"}
						errData, _ := json.Marshal(errMsg)
						
						if mu, ok := broadcaster.WriteMu[conn]; ok {
							mu.Lock()
							conn.WriteMessage(websocket.TextMessage, errData)
							mu.Unlock()
						}
					}

				case "toggle_pause":
					broadcaster.TogglePause()
				}
			}
		}
	}
}