package esphome

import (
	"bytes"
	"image"
	"image/jpeg"
	"time"

	"github.com/golang/protobuf/proto"
	"maze.io/x/esphome/api"
)

// Camera is an ESP32 camera.
type Camera struct {
	Entity
	lastFrame time.Time
}

func newCamera(client *Client, entity *api.ListEntitiesCameraResponse) *Camera {
	return &Camera{
		Entity: Entity{
			Name:     entity.Name,
			ObjectID: entity.ObjectId,
			UniqueID: entity.UniqueId,
			Key:      entity.Key,
			client:   client,
		},
	}
}

// Image grabs one image frame from the camera.
func (entity *Camera) Image() (image.Image, error) {
	if err := entity.client.sendTimeout(&api.CameraImageRequest{
		Stream: true,
	}, entity.client.Timeout); err != nil {
		return nil, err
	}

	var (
		in  = make(chan proto.Message, 1)
		out = make(chan []byte)
	)
	entity.client.waitMutex.Lock()
	entity.client.wait[api.CameraImageResponseType] = in
	entity.client.waitMutex.Unlock()

	go func(in <-chan proto.Message, out chan []byte) {
		for message := range in {
			if message, ok := message.(*api.CameraImageResponse); ok {
				out <- message.Data
				if message.Done {
					close(out)
					entity.lastFrame = time.Now()
					return
				}
			}
		}
	}(in, out)

	var buffer = new(bytes.Buffer)
	for chunk := range out {
		buffer.Write(chunk)
	}

	entity.client.waitMutex.Lock()
	delete(entity.client.wait, api.CameraImageResponseType)
	entity.client.waitMutex.Unlock()

	return jpeg.Decode(buffer)
}

// Stream returns a channel with raw image frame buffers.
func (entity *Camera) Stream() (<-chan *bytes.Buffer, error) {
	if err := entity.client.sendTimeout(&api.CameraImageRequest{
		Stream: true,
	}, entity.client.Timeout); err != nil {
		return nil, err
	}

	in := make(chan proto.Message, 1)
	entity.client.waitMutex.Lock()
	entity.client.wait[api.CameraImageResponseType] = in
	entity.client.waitMutex.Unlock()

	out := make(chan *bytes.Buffer)
	go func(in <-chan proto.Message, out chan<- *bytes.Buffer) {
		var (
			ticker = time.NewTicker(time.Second)
			buffer = new(bytes.Buffer)
		)
		defer ticker.Stop()
		for {
			select {
			case message := <-in:
				frame := message.(*api.CameraImageResponse)
				buffer.Write(frame.Data)
				if frame.Done {
					out <- buffer
					buffer = new(bytes.Buffer)
					entity.lastFrame = time.Now()
				}

			case <-ticker.C:
				if err := entity.client.sendTimeout(&api.CameraImageRequest{
					Stream: true,
				}, entity.client.Timeout); err != nil {
					close(out)
					return
				}
			}
		}
	}(in, out)

	return out, nil
}

// ImageStream is like Stream, returning decoded frame images.
func (entity *Camera) ImageStream() (<-chan image.Image, error) {
	in, err := entity.Stream()
	if err != nil {
		return nil, err
	}

	out := make(chan image.Image)
	go func(in <-chan *bytes.Buffer, out chan image.Image) {
		defer close(out)
		for frame := range in {
			if i, err := jpeg.Decode(frame); err == nil {
				out <- i
			}
		}
	}(in, out)

	return out, nil
}

// LastFrame returns the time of the last camera frame received.
func (entity *Camera) LastFrame() time.Time {
	return entity.lastFrame
}
