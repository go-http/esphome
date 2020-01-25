package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	"maze.io/x/esphome/cmd"
)

func main() {
	flag.Parse()

	client, err := cmd.Dial()
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	var (
		entities = client.Entities()
		w        = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.Debug)
	)
	for _, item := range entities.BinarySensor {
		fmt.Fprintf(w, "binary sensor\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Camera {
		fmt.Fprintf(w, "climate sensor\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Climate {
		fmt.Fprintf(w, "climate sensor\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Cover {
		fmt.Fprintf(w, "cover\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Fan {
		fmt.Fprintf(w, "fan\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Light {
		fmt.Fprintf(w, "light\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Sensor {
		fmt.Fprintf(w, "sensor\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Switch {
		fmt.Fprintf(w, "switch\t%s\t%s\n", item.ObjectID, item.Name)
	}
	for _, item := range entities.Sensor {
		fmt.Fprintf(w, "text sensor\t%s\t%s\n", item.ObjectID, item.Name)
	}
	w.Flush()
}
