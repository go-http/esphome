package main

import (
	"flag"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"strings"

	"maze.io/x/esphome/cmd"
)

func main() {
	var output = flag.String("output", "camera.jpg", "output file")
	flag.Parse()

	log.Printf("connecting to node %s:%s", *cmd.NodeFlag, *cmd.PortFlag)
	client, err := cmd.Dial()
	if err != nil {
		log.Fatalln(err)
	}
	defer client.Close()

	camera, err := client.Camera()
	if err != nil {
		log.Fatalln(err)
	}

	log.Println("requesting camera image")
	i, err := camera.Image()
	if err != nil {
		log.Fatalln(err)
	}

	o, err := os.Create(*output)
	if err != nil {
		log.Fatalln(err)
	}
	defer o.Close()

	switch strings.ToLower(*output) {
	case ".gif":
		err = gif.Encode(o, i, nil)
	case ".png":
		err = png.Encode(o, i)
	default:
		fallthrough
	case ".jpg", ".jpeg":
		err = jpeg.Encode(o, i, nil)
	}

	if err != nil {
		log.Fatalln(err)
	} else if err = o.Close(); err != nil {
		log.Fatalln(err)
	}

	size := i.Bounds().Size()
	log.Printf("saved %dx%d image to %s\n", size.X, size.Y, *output)
}
