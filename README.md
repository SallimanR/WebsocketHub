<h1>
<p align="center">
  WebsocketHub
</h1>

## [ENGLISH README](./README_EN.md)

## О проекте
WebsocketHub — это standalone-сервер и Go-библиотека, позволяющая регистрировать каналы с определённым типом сообщений для конкретных ролей.

## Архитектура
### Неблокирующий I/O
- Чтение и запись сообщений через WebSocket полностью неблокирующие
- Нет двух goroutine (read/write) для каждого подключения: `NBIO` использует epoll (Linux) / kqueue (BSD, macOS), что радикально снижает потребление памяти и не нагружает планировщик Go:

```
  +-------------------+
  |       Клиент      |
  +---------+---------+
            |
            v (HTTP / TCP)
  +-------------------+
  |       Epoll       | (ожидание на уровне ядра)
  +---------+---------+
            |
    (событие EPOLLIN)
            |
            v
  +-------------------+
  |  Пул обработчиков | (фиксированное число goroutines)
  +-------------------+
```
##### В теории масштабируется до **1 миллиона+ соединений**
- [`NBIO`: бенчмарк 1 млн WebSocket-соединений](https://github.com/lesismal/nbio#1m-websocket-connections-benchmark)
- [Time wheel для миллионов конкурентных задач](https://dev.to/kevwan/implement-a-timing-wheel-for-millions-of-concurrent-tasks-30oi)
- [Миллион WebSocket и Go](https://habr.com/ru/companies/vk/articles/331784/) — Сергей Камардин, разработчик в Mail.Ru / VK

### Роли
- WebsocketHub оперирует набором ролей, под которые регистрируются каналы

### Каналы
WebsocketHub построен на каналах, ролях и RBAC (авторизация по ролям):
- каждый канал имеет свой тип сообщений
- каждый канал реализует интерфейс ChannelActions тем самым отделяясь от кода основного сервера
- каждый канал регистрируется на определённый набор ролей

### Сообщения
- WebsocketHub использует Protobuf для сообщений
- формат: запрос/ответ
- типы сообщений: Publish, Subscribe, Unsubscribe

### Планировщик сообщений
- WebsocketHub имеет планировщик, который распределяет рассылку сообщений каждому пользователю через равные промежутки времени (тики)
- используется структура данных [time wheel](./pkg/data_structures/timewheel/time_wheel.go)

## Почему WebSocket?
WebSocket поддерживается всеми основными браузерами.

WebSocket vs REST:
1. не нужен HTTP-сервер и балансировщик нагрузки
2. двунаправленная связь
3. поддерживает бинарный формат сообщений (эффективнее текстового JSON)
4. сохраняет состояние соединения — можно кэшировать подписки на сервере

## Использование как standalone-сервера
#### Установка бинарника:
```sh
go install github.com/SallimanR/websockethub@latest
```
#### Запуск сервера:
```sh
websockethub -config websockethub.json
```

Пример конфигурации:
```json
{
  "port": 8080,
  "roles": ["tow_driver", "tow_subscriber"],
  "channels": [
    { "name": "GPS_REALTIME", "roles": ["tow_driver", "tow_subscriber"] }
  ]
}
```

## Использование как библиотеки
#### Установка:
```sh
go get github.com/SallimanR/websockethub
```

Реальный пример использования: [gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/internal/domains/geolocation/interface/websocket/gps_realtime_channel.go)

[Тесты gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/test/integration/gps_realtime_test.go)

[Бенчмарк gps_realtime_channel](https://github.com/SallimanR/GeoMove_Public/blob/main/backend/monolith/test/integration/gps_realtime_bench_test.go)

Пример с Gin-роутером, CORS и auth-middleware:
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
#### Регистрация канала со своим типом сообщений
Пример с каналом GPS в реальном времени:
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

## Разработка
#### Режим разработки:
запуск сервера с hot code reload:
```sh
air
```

#### Продакшн:
запуск из корня проекта:
```sh
go run .
```
Или сборка бинарника:
```sh
go build -o ./bin/websockethub .
```

#### Запуск тестов:
```sh
go test ./...
```

#### Генерация protobuf:
```sh
generate_protobuf.sh
```
