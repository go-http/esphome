package esphome

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"

	proto "github.com/golang/protobuf/proto"

	"maze.io/x/esphome/api"
)

// Client defaults.
const (
	DefaultTimeout    = 10 * time.Second
	DefaultPort       = 6053
	defaultClientInfo = "maze.io go/esphome"
)

// Client for an ESPHome device.
type Client struct {
	// Info identifies this device with the ESPHome node.
	Info string

	// Timeout for read and write operations.
	Timeout time.Duration

	// Clock returns the current time.
	Clock func() time.Time

	conn        net.Conn
	br          *bufio.Reader
	entities    clientEntities
	err         error
	in          chan proto.Message
	stop        chan struct{}
	waitMutex   sync.RWMutex
	wait        map[uint64]chan proto.Message
	lastMessage time.Time
}

type clientEntities struct {
	binarySensor map[uint32]*BinarySensor
	camera       map[uint32]*Camera
	climate      map[uint32]*Climate
	cover        map[uint32]*Cover
	fan          map[uint32]*Fan
	light        map[uint32]*Light
	sensor       map[uint32]*Sensor
	switches     map[uint32]*Switch
	textSensor   map[uint32]*TextSensor
}

func newClientEntities() clientEntities {
	return clientEntities{
		binarySensor: make(map[uint32]*BinarySensor),
		camera:       make(map[uint32]*Camera),
		climate:      make(map[uint32]*Climate),
		cover:        make(map[uint32]*Cover),
		fan:          make(map[uint32]*Fan),
		light:        make(map[uint32]*Light),
		sensor:       make(map[uint32]*Sensor),
		switches:     make(map[uint32]*Switch),
		textSensor:   make(map[uint32]*TextSensor),
	}
}

// Dial connects to ESPHome native API on the supplied TCP address.
func Dial(addr string) (*Client, error) {
	return DialTimeout(addr, 0)
}

// DialTimeout is like Dial with a custom timeout.
func DialTimeout(addr string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}

	c := &Client{
		Timeout:  timeout,
		Info:     defaultClientInfo,
		Clock:    func() time.Time { return time.Now() },
		conn:     conn,
		br:       bufio.NewReader(conn),
		in:       make(chan proto.Message, 16),
		wait:     make(map[uint64]chan proto.Message),
		stop:     make(chan struct{}),
		entities: newClientEntities(),
	}
	go c.reader()
	return c, nil
}

func (c *Client) reader() {
	defer c.conn.Close()
	for {
		select {
		case <-c.stop:
			return

		default:
			if err := c.readMessage(); err != nil {
				c.err = err
				return
			}
		}
	}
}

func (c *Client) nextMessage() (proto.Message, error) {
	if c.err != nil {
		return nil, c.err
	}
	return <-c.in, nil
}

func (c *Client) nextMessageTimeout(timeout time.Duration) (proto.Message, error) {
	if c.err != nil {
		return nil, c.err
	}
	select {
	case message := <-c.in:
		return message, nil
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}

func (c *Client) waitFor(messageType uint64, in chan proto.Message) {
	c.waitMutex.Lock()
	{
		if other, waiting := c.wait[messageType]; waiting {
			other <- nil
			close(other)
		}
		c.wait[messageType] = in
	}
	c.waitMutex.Unlock()
}

func (c *Client) waitDone(messageType uint64) {
	c.waitMutex.Lock()
	{
		delete(c.wait, messageType)
	}
	c.waitMutex.Unlock()
}

func (c *Client) waitMessage(messageType uint64) proto.Message {
	in := make(chan proto.Message, 1)
	c.waitFor(messageType, in)
	message := <-in
	c.waitDone(messageType)
	return message
}

func (c *Client) waitMessageTimeout(messageType uint64, timeout time.Duration) (proto.Message, error) {
	in := make(chan proto.Message, 1)
	c.waitFor(messageType, in)
	defer c.waitDone(messageType)
	select {
	case message := <-in:
		return message, nil
	case <-time.After(timeout):
		return nil, ErrTimeout
	}
}

func (c *Client) readMessage() (err error) {
	var message proto.Message
	if message, err = api.ReadMessage(c.br); err == nil {
		c.lastMessage = time.Now()
		if !c.handleInternal(message) {
			c.waitMutex.Lock()
			in, waiting := c.wait[api.TypeOf(message)]
			c.waitMutex.Unlock()
			if waiting {
				in <- message
			} else {
				c.in <- message
			}
		}
	}
	return
}

func (c *Client) handleInternal(message proto.Message) bool {
	switch message := message.(type) {
	case *api.DisconnectRequest:
		_ = c.sendTimeout(&api.DisconnectResponse{}, c.Timeout)
		c.Close()
		return true

	case *api.PingRequest:
		_ = c.sendTimeout(&api.PingResponse{}, c.Timeout)
		return true

	case *api.GetTimeRequest:
		_ = c.sendTimeout(&api.GetTimeResponse{EpochSeconds: uint32(c.Clock().Unix())}, c.Timeout)
		return true

	case *api.FanStateResponse:
		if _, ok := c.entities.fan[message.Key]; ok {
			// TODO
			// entity.update(message)
		}
	case *api.CoverStateResponse:
		if _, ok := c.entities.cover[message.Key]; ok {
			// TODO
			// entity.update(message)
		}
	case *api.LightStateResponse:
		if entity, ok := c.entities.light[message.Key]; ok {
			entity.update(message)
		}
	case *api.SensorStateResponse:
		if entity, ok := c.entities.sensor[message.Key]; ok {
			entity.update(message)
		}
	case *api.SwitchStateResponse:
		if entity, ok := c.entities.switches[message.Key]; ok {
			entity.update(message)
		}
	}

	return false
}

func (c *Client) send(message proto.Message) error {
	packed, err := api.Marshal(message)
	if err != nil {
		return err
	}
	if _, err = c.conn.Write(packed); err != nil {
		return err
	}
	return nil
}

func (c *Client) sendTimeout(message proto.Message, timeout time.Duration) error {
	packed, err := api.Marshal(message)
	if err != nil {
		return err
	}
	if err = c.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if _, err = c.conn.Write(packed); err != nil {
		return err
	}
	if err = c.conn.SetWriteDeadline(time.Time{}); err != nil {
		return err
	}
	return nil
}

func (c *Client) sendAndWaitResponse(message proto.Message, messageType uint64) (proto.Message, error) {
	if err := c.send(message); err != nil {
		return nil, err
	}
	return c.waitMessage(messageType), nil
}

func (c *Client) sendAndWaitResponseTimeout(message proto.Message, messageType uint64, timeout time.Duration) (proto.Message, error) {
	if timeout <= 0 {
		return c.sendAndWaitResponse(message, messageType)
	}
	if err := c.sendTimeout(message, timeout); err != nil {
		return nil, err
	}
	return c.waitMessageTimeout(messageType, timeout)
}

// Login must be called to do the initial handshake. The provided password can be empty.
func (c *Client) Login(password string) error {
	message, err := c.sendAndWaitResponseTimeout(&api.HelloRequest{
		ClientInfo: c.Info,
	}, api.HelloResponseType, c.Timeout)
	if err != nil {
		return err
	}

	if message, err = c.sendAndWaitResponseTimeout(&api.ConnectRequest{
		Password: password,
	}, api.ConnectResponseType, c.Timeout); err != nil {
		return err
	}
	connectResponse := message.(*api.ConnectResponse)
	if connectResponse.InvalidPassword {
		return ErrPassword
	}

	// Query available entities, this allows us to map sensor/actor names to keys.
	entities, err := c.listEntities()
	if err != nil {
		return err
	}
	for _, item := range entities {
		switch item := item.(type) {
		case *api.ListEntitiesBinarySensorResponse:
			c.entities.binarySensor[item.Key] = newBinarySensor(c, item)
		case *api.ListEntitiesCameraResponse:
			c.entities.camera[item.Key] = newCamera(c, item)
		case *api.ListEntitiesClimateResponse:
			c.entities.climate[item.Key] = newClimate(c, item)
		case *api.ListEntitiesCoverResponse:
			c.entities.cover[item.Key] = newCover(c, item)
		case *api.ListEntitiesFanResponse:
			c.entities.fan[item.Key] = newFan(c, item)
		case *api.ListEntitiesLightResponse:
			c.entities.light[item.Key] = newLight(c, item)
		case *api.ListEntitiesSensorResponse:
			c.entities.sensor[item.Key] = newSensor(c, item)
		case *api.ListEntitiesSwitchResponse:
			c.entities.switches[item.Key] = newSwitch(c, item)
		case *api.ListEntitiesTextSensorResponse:
			c.entities.textSensor[item.Key] = newTextSensor(c, item)
		default:
			fmt.Printf("unknown\t%T\n", item)
		}
	}

	// Subscribe to states, this is also used for streaming requests.
	if err = c.sendTimeout(&api.SubscribeStatesRequest{}, c.Timeout); err != nil {
		return err
	}

	return nil
}

// Close the device connection.
func (c *Client) Close() error {
	_, err := c.sendAndWaitResponseTimeout(&api.DisconnectRequest{}, api.DisconnectResponseType, 5*time.Second)
	select {
	case c.stop <- struct{}{}:
	default:
	}
	return err
}

// LastMessage returns the time of the last message received.
func (c *Client) LastMessage() time.Time {
	return c.lastMessage
}

// DeviceInfo contains information about the ESPHome node.
type DeviceInfo struct {
	UsesPassword bool

	// The name of the node, given by "App.set_name()"
	Name string

	// The mac address of the device. For example "AC:BC:32:89:0E:A9"
	MacAddress string

	// A string describing the ESPHome version. For example "1.10.0"
	EsphomeVersion string

	// A string describing the date of compilation, this is generated by the compiler
	// and therefore may not be in the same format all the time.
	// If the user isn't using ESPHome, this will also not be set.
	CompilationTime string

	// The model of the board. For example NodeMCU
	Model string

	// HasDeepSleep indicates the device has deep sleep mode enabled when idle.
	HasDeepSleep bool
}

// DeviceInfo queries the ESPHome device information.
func (c *Client) DeviceInfo() (DeviceInfo, error) {
	message, err := c.sendAndWaitResponseTimeout(&api.DeviceInfoRequest{}, api.DeviceInfoResponseType, c.Timeout)
	if err != nil {
		return DeviceInfo{}, err
	}

	info := message.(*api.DeviceInfoResponse)
	return DeviceInfo{
		UsesPassword:    info.UsesPassword,
		Name:            info.Name,
		MacAddress:      info.MacAddress,
		EsphomeVersion:  info.EsphomeVersion,
		CompilationTime: info.CompilationTime,
		Model:           info.Model,
		HasDeepSleep:    info.HasDeepSleep,
	}, nil
}

// Entities returns all configured entities on the connected device.
func (c *Client) Entities() Entities {
	var entities = Entities{
		BinarySensor: make(map[string]*BinarySensor),
		Camera:       make(map[string]*Camera),
		Climate:      make(map[string]*Climate),
		Cover:        make(map[string]*Cover),
		Fan:          make(map[string]*Fan),
		Light:        make(map[string]*Light),
		Sensor:       make(map[string]*Sensor),
		Switch:       make(map[string]*Switch),
		TextSensor:   make(map[string]*TextSensor),
	}
	for _, item := range c.entities.binarySensor {
		entities.BinarySensor[item.UniqueID] = item
	}
	for _, item := range c.entities.camera {
		entities.Camera[item.UniqueID] = item
	}
	for _, item := range c.entities.climate {
		entities.Climate[item.UniqueID] = item
	}
	for _, item := range c.entities.cover {
		entities.Cover[item.UniqueID] = item
	}
	for _, item := range c.entities.fan {
		entities.Fan[item.UniqueID] = item
	}
	for _, item := range c.entities.light {
		entities.Light[item.UniqueID] = item
	}
	for _, item := range c.entities.sensor {
		entities.Sensor[item.UniqueID] = item
	}
	for _, item := range c.entities.switches {
		entities.Switch[item.UniqueID] = item
	}
	for _, item := range c.entities.textSensor {
		entities.TextSensor[item.UniqueID] = item
	}
	return entities
}

// LogLevel represents the logger level.
type LogLevel int32

// Log levels.
const (
	LogNone LogLevel = iota
	LogError
	LogWarn
	LogInfo
	LogDebug
	LogVerbose
	LogVeryVerbose
)

// LogEntry contains a single entry in the ESPHome system log.
type LogEntry struct {
	// Level of the message.
	Level LogLevel

	// Tag for the message.
	Tag string

	// Message is the raw text message.
	Message string

	// SendFailed indicates a failure.
	SendFailed bool
}

// Logs streams log entries.
func (c *Client) Logs(level LogLevel) (chan LogEntry, error) {
	if err := c.sendTimeout(&api.SubscribeLogsRequest{
		Level: api.LogLevel(level),
	}, c.Timeout); err != nil {
		return nil, err
	}

	in := make(chan proto.Message, 1)
	c.waitMutex.Lock()
	c.wait[api.SubscribeLogsResponseType] = in
	c.waitMutex.Unlock()

	out := make(chan LogEntry)
	go func(in <-chan proto.Message, out chan LogEntry) {
		defer close(out)
		for entry := range in {
			entry := entry.(*api.SubscribeLogsResponse)
			out <- LogEntry{
				Level:      LogLevel(entry.Level),
				Tag:        entry.Tag,
				Message:    entry.Message,
				SendFailed: entry.SendFailed,
			}
		}
	}(in, out)

	return out, nil
}

// listEntities lists connected entities.
func (c *Client) listEntities() (entities []proto.Message, err error) {
	if err = c.sendTimeout(&api.ListEntitiesRequest{}, c.Timeout); err != nil {
		return nil, err
	}

	for {
		message, err := c.nextMessageTimeout(c.Timeout)
		if err != nil {
			return nil, err
		}
		switch message := message.(type) {
		case *api.ListEntitiesDoneResponse:
			return entities, nil
		case *api.ListEntitiesBinarySensorResponse,
			*api.ListEntitiesCameraResponse,
			*api.ListEntitiesClimateResponse,
			*api.ListEntitiesCoverResponse,
			*api.ListEntitiesFanResponse,
			*api.ListEntitiesLightResponse,
			*api.ListEntitiesSensorResponse,
			*api.ListEntitiesServicesResponse,
			*api.ListEntitiesSwitchResponse,
			*api.ListEntitiesTextSensorResponse:
			entities = append(entities, message)
		}
	}
}

// Camera returns a reference to the camera. It returns an error if no camera is found.
func (c *Client) Camera() (Camera, error) {
	for _, entity := range c.entities.camera {
		return *entity, nil
	}
	return Camera{}, ErrEntity
}

// Ping the server.
func (c *Client) Ping() error {
	return c.PingTimeout(c.Timeout)
}

// PingTimeout is like ping with a custom timeout.
func (c *Client) PingTimeout(timeout time.Duration) error {
	// ESPHome doesn't respond to ping (bug? expected?), so we fire & forget.
	return c.sendTimeout(&api.PingRequest{}, timeout)
}
