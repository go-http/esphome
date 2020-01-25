package main

import (
	"flag"
	"fmt"
	"log"

	"maze.io/x/esphome"
	"maze.io/x/esphome/cmd"
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
