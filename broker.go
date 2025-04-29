package solBlockSubscribe

import (
	"context"
	"github.com/gagliardetto/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"

	"sync"
)

func NewBlock(endPoint string, ctx context.Context, logger *log.Logger) *Block {
	return &Block{
		endPoint: endPoint,
		Broker: struct {
			sync.Mutex
			subscribers map[chan *ws.BlockResult]struct{}
		}{
			subscribers: make(map[chan *ws.BlockResult]struct{}),
		},
		ctx:    ctx,
		logger: logger,
	}
}

func (b *Block) SubscribeBlock() chan *ws.BlockResult {
	b.Broker.Lock()
	defer b.Broker.Unlock()

	ch := make(chan *ws.BlockResult, 100)
	b.Broker.subscribers[ch] = struct{}{}
	return ch
}

func (b *Block) UnsubscribeBlock(ch chan *ws.BlockResult) {
	b.Broker.Lock()
	defer b.Broker.Unlock()
	if _, ok := b.Broker.subscribers[ch]; ok {
		delete(b.Broker.subscribers, ch)
		close(ch)
	}
}

func (b *Block) publish(blockData *ws.BlockResult) {
	b.Broker.Lock()
	defer b.Broker.Unlock()
	for ch := range b.Broker.subscribers {
		select {
		case ch <- blockData:
		default:
			b.logger.Printf("subscriber channel full, dropping block for one listener")
		}
	}
}
