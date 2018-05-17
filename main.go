package main

import (
	"log"
)

func main() {
	err := startApp()
	if err != nil {
		log.Fatalf("unable to start server: %v", err)
	}
}
