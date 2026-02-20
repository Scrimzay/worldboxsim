package main

import (
	"log"
	"os"

	"github.com/Scrimzay/worldboxsim/internal/server"
	"github.com/Scrimzay/worldboxsim/internal/world"
	//"github.com/gin-gonic/gin"
)
	
func main() {
    log.Println("=== STARTING WORLDBOX SIM ===")
    
    // Init world
    log.Println("Creating world...")
    gameWorld := world.New()
    log.Println("World created!")
    
    // Start broadcaster in background
    log.Println("Creating broadcaster...")
    broadcaster := world.NewBroadcaster(gameWorld)
    log.Println("Starting broadcaster...")
    go broadcaster.Run()
    
    // Get port form env (Koyeb sets this)
    port := os.Getenv("PORT")
    if port == "" {
        port = "8000" // default for koyeb
    }

    // Setup and start server
    log.Println("Setting up router...")
    r := server.SetupRouter(broadcaster, gameWorld)
    log.Printf("Server starting at port %s", port)
    if err := r.Run(":" + port); err != nil {
        log.Fatal("Server failed:", err)
    }
}