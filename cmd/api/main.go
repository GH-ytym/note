package main

import (
	"log"
	_ "time/tzdata"

	"note/internal"
)

func main() {
	if err := internal.Run(); err != nil {
		log.Fatal(err)
	}
}
