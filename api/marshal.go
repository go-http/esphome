package api

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
)

var messageType = map[int]interface{}{
	1:  HelloRequest{},
	2:  HelloResponse{},
	3:  ConnectRequest{},
	4:  ConnectResponse{},
	5:  DisconnectRequest{},
	6:  DisconnectResponse{},
	7:  PingRequest{},
	8:  PingResponse{},
	9:  DeviceInfoRequest{},
	10: DeviceInfoResponse{},
	11: ListEntitiesRequest{},
	12: ListEntitiesBinarySensorResponse{},
	13: ListEntitiesCoverResponse{},
	14: ListEntitiesFanResponse{},
	15: ListEntitiesLightResponse{},
	16: ListEntitiesSensorResponse{},
	17: ListEntitiesSwitchResponse{},
	18: ListEntitiesTextSensorResponse{},
	19: ListEntitiesDoneResponse{},
	20: SubscribeStatesRequest{},
	21: BinarySensorStateResponse{},
	22: CoverStateResponse{},
	23: FanStateResponse{},
	24: LightStateResponse{},
	25: SensorStateResponse{},
	26: SwitchStateResponse{},
	27: TextSensorStateResponse{},
	28: SubscribeLogsRequest{},
	29: SubscribeLogsResponse{},
	30: CoverCommandRequest{},
	31: FanCommandRequest{},
	32: LightCommandRequest{},
	33: SwitchCommandRequest{},
	34: SubscribeHomeassistantServicesRequest{},
	35: HomeassistantServiceResponse{},
	36: GetTimeRequest{},
	37: GetTimeResponse{},
	38: SubscribeHomeAssistantStatesRequest{},
	39: SubscribeHomeAssistantStateResponse{},
	40: HomeAssistantStateResponse{},
	41: ListEntitiesServicesResponse{},
	42: ExecuteServiceRequest{},
	43: ListEntitiesCameraResponse{},
	44: CameraImageResponse{},
	45: CameraImageRequest{},
	46: ListEntitiesClimateResponse{},
	47: ClimateStateResponse{},
	48: ClimateCommandRequest{},
}

func Marshal(message proto.Message) ([]byte, error) {
	encoded, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}

	var (
		packed = make([]byte, len(encoded)+17)
		n      = 1
	)

	// Write encoded message length
	n += binary.PutUvarint(packed[n:], uint64(len(encoded)))

	// Write message type
	n += binary.PutUvarint(packed[n:], TypeOf(message))

	// Write message
	copy(packed[n:], encoded)
	n += len(encoded)

	return packed[:n], nil
}

func ReadMessage(r *bufio.Reader) (proto.Message, error) {
	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != 0x00 {
		return nil, errors.New("api: protocol error: expected null byte")
	}

	// Read encoded message length
	length, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}

	// Read encoded message type
	kind, err := binary.ReadUvarint(r)
	if err != nil {
		return nil, err
	}

	// Read encoded message
	encoded := make([]byte, length)
	if _, err = io.ReadFull(r, encoded); err != nil {
		return nil, err
	}

	message := newMessage(kind)
	if message == nil {
		return nil, fmt.Errorf("api: protocol error: unknown message type %#x", kind)
	}

	if err = proto.Unmarshal(encoded, message); err != nil {
		return nil, err
	}
	return message, nil
}

func TypeOf(value interface{}) uint64 {
	switch value.(type) {
	case HelloRequest, *HelloRequest:
		return HelloRequestType
	case HelloResponse, *HelloResponse:
		return HelloResponseType
	case ConnectRequest, *ConnectRequest:
		return ConnectRequestType
	case ConnectResponse, *ConnectResponse:
		return ConnectResponseType
	case DisconnectRequest, *DisconnectRequest:
		return DisconnectRequestType
	case DisconnectResponse, *DisconnectResponse:
		return DisconnectResponseType
	case PingRequest, *PingRequest:
		return PingRequestType
	case PingResponse, *PingResponse:
		return PingResponseType
	case DeviceInfoRequest, *DeviceInfoRequest:
		return DeviceInfoRequestType
	case DeviceInfoResponse, *DeviceInfoResponse:
		return DeviceInfoResponseType
	case ListEntitiesRequest, *ListEntitiesRequest:
		return ListEntitiesRequestType
	case ListEntitiesBinarySensorResponse, *ListEntitiesBinarySensorResponse:
		return ListEntitiesBinarySensorResponseType
	case ListEntitiesCoverResponse, *ListEntitiesCoverResponse:
		return ListEntitiesCoverResponseType
	case ListEntitiesFanResponse, *ListEntitiesFanResponse:
		return ListEntitiesFanResponseType
	case ListEntitiesLightResponse, *ListEntitiesLightResponse:
		return ListEntitiesLightResponseType
	case ListEntitiesSensorResponse, *ListEntitiesSensorResponse:
		return ListEntitiesSensorResponseType
	case ListEntitiesSwitchResponse, *ListEntitiesSwitchResponse:
		return ListEntitiesSwitchResponseType
	case ListEntitiesTextSensorResponse, *ListEntitiesTextSensorResponse:
		return ListEntitiesTextSensorResponseType
	case ListEntitiesDoneResponse, *ListEntitiesDoneResponse:
		return ListEntitiesDoneResponseType
	case SubscribeStatesRequest, *SubscribeStatesRequest:
		return SubscribeStatesRequestType
	case BinarySensorStateResponse, *BinarySensorStateResponse:
		return BinarySensorStateResponseType
	case CoverStateResponse, *CoverStateResponse:
		return CoverStateResponseType
	case FanStateResponse, *FanStateResponse:
		return FanStateResponseType
	case LightStateResponse, *LightStateResponse:
		return LightStateResponseType
	case SensorStateResponse, *SensorStateResponse:
		return SensorStateResponseType
	case SwitchStateResponse, *SwitchStateResponse:
		return SwitchStateResponseType
	case TextSensorStateResponse, *TextSensorStateResponse:
		return TextSensorStateResponseType
	case SubscribeLogsRequest, *SubscribeLogsRequest:
		return SubscribeLogsRequestType
	case SubscribeLogsResponse, *SubscribeLogsResponse:
		return SubscribeLogsResponseType
	case CoverCommandRequest, *CoverCommandRequest:
		return CoverCommandRequestType
	case FanCommandRequest, *FanCommandRequest:
		return FanCommandRequestType
	case LightCommandRequest, *LightCommandRequest:
		return LightCommandRequestType
	case SwitchCommandRequest, *SwitchCommandRequest:
		return SwitchCommandRequestType
	case SubscribeHomeassistantServicesRequest, *SubscribeHomeassistantServicesRequest:
		return SubscribeHomeAssistantServicesRequestType
	case HomeassistantServiceResponse, *HomeassistantServiceResponse:
		return HomeAssistantServiceResponseType
	case GetTimeRequest, *GetTimeRequest:
		return GetTimeRequestType
	case GetTimeResponse, *GetTimeResponse:
		return GetTimeResponseType
	case SubscribeHomeAssistantStatesRequest, *SubscribeHomeAssistantStatesRequest:
		return SubscribeHomeAssistantStatesRequestType
	case SubscribeHomeAssistantStateResponse, *SubscribeHomeAssistantStateResponse:
		return SubscribeHomeAssistantStateResponseType
	case HomeAssistantStateResponse, *HomeAssistantStateResponse:
		return HomeAssistantStateResponseType
	case ListEntitiesServicesResponse, *ListEntitiesServicesResponse:
		return ListEntitiesServicesResponseType
	case ExecuteServiceRequest, *ExecuteServiceRequest:
		return ExecuteServiceRequestType
	case ListEntitiesCameraResponse, *ListEntitiesCameraResponse:
		return ListEntitiesCameraResponseType
	case CameraImageResponse, *CameraImageResponse:
		return CameraImageResponseType
	case CameraImageRequest, *CameraImageRequest:
		return CameraImageRequestType
	case ListEntitiesClimateResponse, *ListEntitiesClimateResponse:
		return ListEntitiesClimateResponseType
	case ClimateStateResponse, *ClimateStateResponse:
		return ClimateStateResponseType
	case ClimateCommandRequest, *ClimateCommandRequest:
		return ClimateCommandRequestType
	default:
		return UnknownType
	}
}

func newMessage(kind uint64) proto.Message {
	switch kind {
	case 1:
		return new(HelloRequest)
	case 2:
		return new(HelloResponse)
	case 3:
		return new(ConnectRequest)
	case 4:
		return new(ConnectResponse)
	case 5:
		return new(DisconnectRequest)
	case 6:
		return new(DisconnectResponse)
	case 7:
		return new(PingRequest)
	case 8:
		return new(PingResponse)
	case 9:
		return new(DeviceInfoRequest)
	case 10:
		return new(DeviceInfoResponse)
	case 11:
		return new(ListEntitiesRequest)
	case 12:
		return new(ListEntitiesBinarySensorResponse)
	case 13:
		return new(ListEntitiesCoverResponse)
	case 14:
		return new(ListEntitiesFanResponse)
	case 15:
		return new(ListEntitiesLightResponse)
	case 16:
		return new(ListEntitiesSensorResponse)
	case 17:
		return new(ListEntitiesSwitchResponse)
	case 18:
		return new(ListEntitiesTextSensorResponse)
	case 19:
		return new(ListEntitiesDoneResponse)
	case 20:
		return new(SubscribeStatesRequest)
	case 21:
		return new(BinarySensorStateResponse)
	case 22:
		return new(CoverStateResponse)
	case 23:
		return new(FanStateResponse)
	case 24:
		return new(LightStateResponse)
	case 25:
		return new(SensorStateResponse)
	case 26:
		return new(SwitchStateResponse)
	case 27:
		return new(TextSensorStateResponse)
	case 28:
		return new(SubscribeLogsRequest)
	case 29:
		return new(SubscribeLogsResponse)
	case 30:
		return new(CoverCommandRequest)
	case 31:
		return new(FanCommandRequest)
	case 32:
		return new(LightCommandRequest)
	case 33:
		return new(SwitchCommandRequest)
	case 34:
		return new(SubscribeHomeassistantServicesRequest)
	case 35:
		return new(HomeassistantServiceResponse)
	case 36:
		return new(GetTimeRequest)
	case 37:
		return new(GetTimeResponse)
	case 38:
		return new(SubscribeHomeAssistantStatesRequest)
	case 39:
		return new(SubscribeHomeAssistantStateResponse)
	case 40:
		return new(HomeAssistantStateResponse)
	case 41:
		return new(ListEntitiesServicesResponse)
	case 42:
		return new(ExecuteServiceRequest)
	case 43:
		return new(ListEntitiesCameraResponse)
	case 44:
		return new(CameraImageResponse)
	case 45:
		return new(CameraImageRequest)
	case 46:
		return new(ListEntitiesClimateResponse)
	case 47:
		return new(ClimateStateResponse)
	case 48:
		return new(ClimateCommandRequest)
	default:
		return nil
	}
}
