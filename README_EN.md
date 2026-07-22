<h1>
<p align="center">
  WebsocketHub
</h1>

## About
WebsocketHub is a standalone server / Go library that supports registering 
channels with specific message type for specific roles 

## Architecture
### Non-blocking I/O
- Message reads and writes to websocket connection are fully non-blocking
- No goroutine per connection: `NBIO` uses epoll(Linux) / kqueue(BSD based OSs, MacOS)
which means that memory usage is way lower per connection and Go's scheduler is not loaded
```
  +-------------------+
  | Client Connection |
  +---------+---------+
            |
            v (HTTP / raw TCP)
  +-------------------+
  |   Epoll Instance  | (kernel-level wait)
  +---------+---------+
            |
(Trigger on EPOLLIN event)
            |
            v
  +-------------------+
  |   Worker Pool     | (fixed size goroutines)
  +-------------------+
```
##### In theory it scales up to **1 Million+ connections**
- [`NBIO` websocket library 1 Million connections benchmark](https://github.com/lesismal/nbio#1m-websocket-connections-benchmark)
- [time wheel for millions of concurrent tasks](https://dev.to/kevwan/implement-a-timing-wheel-for-millions-of-concurrent-tasks-30oi)
- [Million WebSockets and Go](https://habr.com/ru/companies/vk/articles/331784/) — Sergey Kamardin, developer in Mail.Ru / VK

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
3. supports binary format for messages (more effective then text based JSON)
4. stateful => ability to cache subscriptions on server side

## Usage as standalone server
#### install the binary:
```sh
go install github.com/SallimanR/websockethub@latest
```
#### run the server:
```sh
websockethub -config websockethub.json
```

Example config:
```json
{
  "port": 8080,
  "roles": ["tow_driver", "tow_subscriber"],
  "channels": [
    { "name": "GPS_REALTIME", "roles": ["tow_driver", "tow_subscriber"] }
  ]
}
```

## Usage as library
#### install the library:
```sh
go get github.com/SallimanR/websockethub
```

For realworld example see [gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/internal/domains/geolocation/interface/websocket/gps_realtime_channel.go)

[Tests for gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/test/integration/gps_realtime_test.go)

[Benchmark for gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/test/integration/gps_realtime_bench_test.go)

example with gin router, allowed origins for CORS protection and auth middleware
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
#### register channel with message type
example with gps realtime channel
```go
import (
    "time"

    "github.com/SallimanR/websockethub/websockethub"
    wsPB "github.com/SallimanR/websockethub/websockethub/proto"
    "google.golang.org/protobuf/proto"
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

type GPSRealtimeChannel struct {
    *websockethub.PubSubChannel[MovingDriverWithPoints]
}

func NewGPSRealtimeChannel(wsServer *websockethub.WebsocketServer, roles []string) (*GPSRealtimeChannel, error) {
    ch := &GPSRealtimeChannel{
        PubSubChannel: websockethub.NewPubSubChannel[MovingDriverWithPoints](),
    }
    err := wsServer.RegisterChannel(roles, wsPB.Channel_GPS_REALTIME, ch)
    return ch, err
}

func (c *GPSRealtimeChannel) Publish(publisherID int64, msg []byte) error {
    var data wsPB.GPSUpdate
    if err := proto.Unmarshal(msg, &data); err != nil {
        return err
    }
    gpsData := MovingDriverWithPoints{
        MovingDriver: MovingDriver{
            DriverID:   publisherID,
            Latitude:   data.Coordinates[0].Lat,
            Longitude:  data.Coordinates[0].Lng,
            TravelTime: time.Now(),
            PathMeters: int32(len(data.Coordinates)),
        },
    }
    c.Messages.Store(publisherID, gpsData)
    return nil
}

func (c *GPSRealtimeChannel) GetMessages(publisherIDs []int64) ([]byte, error) {
    var batch wsPB.MessageBatch
    for _, id := range publisherIDs {
        item, ok := c.Messages.Load(id)
        if !ok {
            continue
        }
        data, err := proto.Marshal(&item)
        if err != nil {
            return nil, err
        }
        batch.Data = append(batch.Data, data)
    }
    return proto.Marshal(&batch)
}
```

## Developing
#### Development mode:
run server with hot code reload:
```sh
air
```

#### Production:
run in the project root:
```sh
go run .
```
Or build the binary:
```sh
go build -o ./bin/websockethub .
```

#### Running tests:
```sh
go test ./...
```

#### generate protobuf
```sh
generate_protobuf.sh

```
