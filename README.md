<h1>
<p align="center">
  WebsocketHub
</h1>

## About
WebsocketHub is a standalone server / Go library that supports registering 
channels with specific message type for specific roles

## Usage as library
### install the library:
```sh
go get github.com/SallimanR/websockethub
```

### with gin router, allowed origins for CORS protection and auth middleware
```go
import (
    "github.com/SallimanR/websockethub/websockethub"
    "github.com/gin-gonic/gin"
)

func registerWSRoutes(router *gin.Engine, origins []string, authMiddleware gin.HandlerFunc) {
    wsOptions := websockethub.WebsocketServerOptions{
        Roles:          []string{"tow_driver", "tow_subscriber"},
        AllowedOrigins: origins,
    }
    wsServer := websockethub.NewWebsocketServer(wsOptions)
	wsGroup := router.Group("/ws")
	wsGroup.Use(authMiddleware)
	wsGroup.GET("/:role", wsServer.WebsocketUpgradeHandler)
}
```
### register channel with message type
example with gps realtime channel
```go
import (
    "time"

    "github.com/SallimanR/websockethub/websockethub"
    wsPB "github.com/SallimanR/websockethub/websockethub/proto"
)

type MovingDriver struct {
	DriverID   int64
	Latitude   float32
	Longitude  float32
	TravelTime time.Time
	PathMeters int32
}

type MovingDriverWithPoints struct {
	MovingDriver
	Points [][2]float32
}

type GPSRealtimeChannel = *websockethub.PubSubChannel[entity.MovingDriverWithPoints]

func NewGPSRealtimeChannel(wsServer *websockethub.WebsocketServer, roles []string) (*GPSRealtimeChannel, wsPB.Channel, error) {
	const channelName = wsPB.Channel_GPS_REALTIME
    channel := websockethub.NewPubSubChannel[entity.MovingDriverWithPoints]()
	err := wsServer.RegisterChannel(roles, channelName, channel)
	if err != nil {
		return nil, channelName, err
	}
	return channel, channelName, nil
}
```

## Developing
### Development mode:
run server with hot code reload:
```sh
air
```

### Production:
run in the project root:
```sh
go run ./cmd/main.go
```

### Running tests:
```sh
go test ./...
```

### generate protobuf
```sh
generate_protobuf.sh

```


## Architecture
### Non-blocking I/O
- Message reads and writes to websocket connection are fully non-blocking
- No goroutine per connection: NBIO uses epoll(Linux) / kqueue(BSD based OSs, MacOS)
which means that memory usage is way lower per connection and Go's scheduler is not loaded
so in theory it scales up to 1 Million+ connections

### Roles
- WebsocketHub have a set of roles under which channels can be registered
### Channels
WebsocketHub is based on channels, roles and RBAC:
- each channels have its own message type
- each channel is interfaced by ChannelActions interface
- each channel is registered to set of roles
### Messages:
- WebsocketHub uses protobuf for messages
- request/response messages
- each message have a type of Publish, Subscribe, Unsubscribe 
### Message scheduler
- WebsocketHub have a message scheduler that schedules each message's broadcast for each user for a constant period of ticks
- it uses a version of [time wheel](./pkg/data_structures/timewheel/time_wheel.go) data structure

## Why Websocket?
Websocket is supported in all major browsers
Websocket vs REST:
1. no need for http server and load balancer
2. bidirectional
3. supports binary format for messages
4. stateful => ability to cache subscriptions on server side
