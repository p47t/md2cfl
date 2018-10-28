package main

import (
	"github.com/p47t/md2cfl/commands"
	"log"
)

func main() {
	if err := commands.Execute(); err != nil {
		log.Fatalln(err)
	}
}
