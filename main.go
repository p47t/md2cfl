package main

import (
	"log"

	"github.com/p47t/md2cfl/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		log.Fatalln(err)
	}
}
