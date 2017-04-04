package main

import (
	"context"
	"log"
	"os"
	"sync"
	"time"

	"golang.skroutz.gr/skroutz/rafka/kafka"
)

type ConsumerID string
type consumerPoolEntry struct {
	consumer *kafka.Consumer
	cancel   context.CancelFunc
}

type consumerPool map[ConsumerID]*consumerPoolEntry

type Manager struct {
	pool consumerPool
	log  *log.Logger
	wg   sync.WaitGroup
	mu   sync.Mutex
	ctx  context.Context
}

func NewManager(ctx context.Context) *Manager {
	c := Manager{
		log:  log.New(os.Stderr, "[manager] ", log.Ldate|log.Ltime),
		pool: make(consumerPool),
		ctx:  ctx,
	}

	return &c
}

func (m *Manager) Run() {
	tickGC := time.NewTicker(10 * time.Second)
	defer tickGC.Stop()

	for {
		select {
		case <-m.ctx.Done():
			m.log.Println("Shutting down...")
			m.cleanup()
			return
		case <-tickGC.C:
			m.reapStale()
		}
	}
}

func (m *Manager) Get(id ConsumerID) *kafka.Consumer {
	// Create consumer if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.pool[id]; !ok {
		// TODO fix topic name
		c := kafka.NewConsumer([]string{"http_bots"}, &kafkacfg)
		ctx, cancel := context.WithCancel(m.ctx)
		m.pool[id] = &consumerPoolEntry{
			consumer: c,
			cancel:   cancel,
		}
		m.log.Printf("Spawning Consumer id:%s config:%v", id, kafkacfg)
		m.wg.Add(1)
		go func(ctx context.Context) {
			defer m.wg.Done()
			c.Run(ctx)
		}(ctx)
	}

	return m.pool[id].consumer
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, entry := range m.pool {
		m.log.Printf("Terminating consumer id:%s", id)
		entry.cancel()
	}

	m.log.Println("Waiting all consumers to finish...")
	m.wg.Wait()
	m.log.Println("All consumers shut down, bye!")
}

func (m *Manager) reapStale() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, _ := range m.pool {
		// TODO actual cleanup
		m.log.Printf("Cleaning up: id:%s", id)
	}
	return
}
