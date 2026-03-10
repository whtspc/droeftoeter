package main

import (
	"log"

	"github.com/whtspc/droeftoeter/config"
	"github.com/whtspc/droeftoeter/ui"
)

func main() {
	cfg := config.Load()
	if err := ui.Run(cfg); err != nil {
		log.Fatal(err)
	}
}
