package server

import (
	"sync"

	wsPB "github.com/SallimanR/websockethub/websockethub/proto"
	"google.golang.org/protobuf/proto"
)

type rawChannel struct {
	mu       sync.RWMutex
	messages map[int64][]byte
}

func newRawChannel() *rawChannel {
	return &rawChannel{messages: make(map[int64][]byte)}
}

func (c *rawChannel) Publish(publisherID int64, msg []byte) error {
	c.mu.Lock()
	c.messages[publisherID] = msg
	c.mu.Unlock()
	return nil
}

func (c *rawChannel) GetMessages(publisherIDs []int64) ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	batch := &wsPB.MessageBatch{}
	for _, id := range publisherIDs {
		if msg, ok := c.messages[id]; ok {
			batch.Data = append(batch.Data, msg)
		}
	}
	return proto.Marshal(batch)
}
