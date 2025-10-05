package main

import (
	"log"

	"goblons/internal/server"
)

func main() {
	srv := server.NewServer()

	log.Println("Starting Goblons multiplayer server...")
	if err := srv.Start(":8080"); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
