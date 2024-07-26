package main

import (
	"hamsterbot/internal/pkg/app"
	"log"
)

func main() {
	_, err := app.New()
	if err != nil {
		log.Fatal(err)
	}
}
