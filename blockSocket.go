package solBlockSubscribe

import (
	"context"
	"fmt"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/ws"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

type Block struct {
	cancel   *context.CancelFunc
	logger   *log.Logger
	endPoint string

	wsClient   *ws.Client
	ctx        context.Context
	httpHeader http.Header

	Broker struct {
		sync.Mutex
		subscribers map[chan *ws.BlockResult]struct{}
	}
	isErr struct {
		onErr bool
		sync.RWMutex
	}
}

func (b *Block) blockStream(httpHeader http.Header) {
	childCtx, cancel := context.WithCancel(b.ctx)
	b.cancel = &cancel
	b.httpHeader = httpHeader
	go func() {
		for {
			select {
			case <-childCtx.Done():
				b.logger.Info("BlockStream done")
				return
			default:
				if err := b.blockSocket(childCtx); err != nil {
					b.logger.Warn("BlockStream reconnecting...")
				} else {
					time.Sleep(time.Second * 1)
					b.logger.Error("BlockStream stop for no reason")
					return
				}
			}
		}
	}()

	return
}

func ptrUint64(v uint64) *uint64 { return &v }

func (b *Block) blockSocket(ctx context.Context) error {
	opts := &ws.Options{
		HttpHeader:       b.httpHeader,
		HandshakeTimeout: time.Duration(10) * time.Second,
	}
	wsClient, err := ws.ConnectWithOptions(ctx, b.endPoint, opts)
	if err != nil {
		return fmt.Errorf("failed to connect to ws: %w", err)
	}
	b.wsClient = wsClient
	defer b.wsClient.Close()

	// 2) subscribe to all blocks
	sub, err := wsClient.BlockSubscribe(
		ws.NewBlockSubscribeFilterAll(),
		&ws.BlockSubscribeOpts{
			Commitment:                     rpc.CommitmentConfirmed,
			Encoding:                       solana.EncodingBase64,
			TransactionDetails:             rpc.TransactionDetailsFull,
			MaxSupportedTransactionVersion: ptrUint64(0), // support v0 txns :contentReference[oaicite:1]{index=1}
		},
	)
	if err != nil {
		b.logger.Printf("failed to subscribe to blocks: %v", err)
		return err
	}
	defer sub.Unsubscribe()
	fmt.Printf("subscribed to block stream")
	// 3) prepare broker
	if b.Broker.subscribers == nil {
		b.Broker.subscribers = make(map[chan *ws.BlockResult]struct{})
	}

	b.setIsErr(false)
	// 4) pump incoming WS messages into a Go channel
	msgCh := make(chan *ws.BlockResult)
	errCh := make(chan error, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				res, err := sub.Recv(ctx)
				if err != nil {
					errCh <- err
					return
				}
				msgCh <- res
			}
		}
	}()

	// 5) main loop: fan-out or error
	for {
		select {
		case <-ctx.Done():
			b.logger.Println("blockSocket context closed")
			return nil
		case err := <-errCh:
			b.logger.Printf("blockSocket error: %v", err)
			b.setIsErr(true)
			return err
		case blk := <-msgCh:
			// publish to all subscribers
			b.publish(blk)
		}
	}
}

// setErr sets the error state
func (b *Block) setIsErr(isErr bool) {
	b.isErr.Lock()
	defer b.isErr.Unlock()
	b.isErr.onErr = isErr
}

// getErr returns the error state
func (b *Block) getIsErr() bool {
	b.isErr.RLock()
	defer b.isErr.RUnlock()
	return b.isErr.onErr
}
