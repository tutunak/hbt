package main

import (
	"fmt"
	"hbt/internal/db"
	"hbt/internal/web"
	"net/http"
	"os"
)

func main() {
	dbPath := db.DefaultPath()
	database, err := db.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer database.Close()

	srv, err := web.NewServer(database)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing server: %v\n", err)
		os.Exit(1)
	}

	port := os.Getenv("HBT_PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	fmt.Printf("hbt listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, srv); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
