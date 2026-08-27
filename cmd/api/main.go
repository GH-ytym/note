package main

import (
	"log"

	"note/internal"
)

func main() {
	if err := internal.Run(); err != nil {
		log.Fatal(err)
	}
}
