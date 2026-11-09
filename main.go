package main

import (
	"log"

	"io.github.tangqiu0205/tiaopin-fm/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
