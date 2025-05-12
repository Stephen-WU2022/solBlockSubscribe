package solBlockSubscribe

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"
)

func manuallyStop(cancel *context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	fmt.Println("Ctrl + C pressed!")

	time.Sleep(time.Second)
	(*cancel)()
}

func TestNewBlock(t *testing.T) {
	// create a new client
	ctx, cancel := context.WithCancel(context.Background())
	logger := log.New()
	block := NewBlock("wss://dimensional-damp-snow.solana-mainnet.quiknode.pro/07ad4b87831307c0820ca68fa0cd37262f3c85af/", ctx, logger)
	block.BlockStream(nil)
	ch := block.SubscribeBlock()
	start := time.Now()
	go func() {
		defer block.UnsubscribeBlock(ch)
		for {
			select {
			case <-ctx.Done():
				return
			case blockResult := <-ch:
				fmt.Println("blockResult", blockResult.Value.Slot)
				// duration
				duration := time.Since(start)
				// print duration in microseconds
				fmt.Printf("duration: %v\n", duration.Milliseconds())
				start = time.Now()
			default:

			}
		}
	}()

	manuallyStop(&cancel)
}
