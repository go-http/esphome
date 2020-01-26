package main

import (
	"log"

	"maze.io/x/esphome"
)

func main() {
	devices := make(chan *esphome.Device, 32)
	go dump(devices)

	if err := esphome.Discover(devices); err != nil {
		log.Fatalln("discovery failed:", err)
	}
}

func dump(devices <-chan *esphome.Device) {
	for device := range devices {
		log.Printf("discovered %s on %s (version %s)",
			device.Name, device.Addr(), device.Version)
	}
}
