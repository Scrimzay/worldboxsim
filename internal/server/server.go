package server

import (
	"log"

	"github.com/Scrimzay/worldboxsim/internal/world"
	"github.com/gin-gonic/gin"
)

func SetupRouter(broadcaster *world.Broadcaster, gameWorld *world.World) *gin.Engine {
	r := gin.Default()
	r.LoadHTMLGlob("**/*.html")
	r.Static("/static", "./static")

	r.GET("/", indexHandler)
	r.GET("/play/:mapName", playHandler(gameWorld, broadcaster))
	r.GET("/help", helpHandler)

	r.GET("/ws", HandleWebsocket(broadcaster, gameWorld))

	return r
}

func indexHandler(c *gin.Context) {
	c.HTML(200, "index.html", nil)
}

func helpHandler(c *gin.Context) {
	c.HTML(200, "help.html", nil)
}

func playHandler(gameWorld *world.World, broadcaster *world.Broadcaster) gin.HandlerFunc {
    return func(c *gin.Context) {
		mapName := c.Param("mapName")
		log.Printf("=== LOADING MAP: %s ===", mapName)
		
		gameWorld.Reset()
		gameWorld.InitMap(mapName)
		broadcaster.BroadcastGrid()
		broadcaster.BroadcastStats()
		
		// Render the appropriate template based on map
		switch mapName {
		case "vertical":
			c.HTML(200, "verticalworld.html", nil)
		case "northsouth":
			c.HTML(200, "northsouthworld.html", nil)
        case "fourquadrants":
            c.HTML(200, "fourquadsworld.html", nil)
		case "custommap":
			c.HTML(200, "customworld.html", nil)
		default:
			c.HTML(200, "index.html", nil)
		}
	}
}