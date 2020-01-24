package main

import (
	"flag"
	"fmt"
	"log"

	"maze.io/esphome/cmd"

	"maze.io/esphome"
)

func main() {
	var level = flag.Int("level", int(esphome.LogVeryVerbose), "entry level (0-6)")
	flag.Parse()

	client, err := cmd.Dial()
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	logs, err := client.Logs(esphome.LogLevel(*level))
	if err != nil {
		log.Fatalln(err)
	}

	for entry := range logs {
		fmt.Println(entry.Message)
	}
}
