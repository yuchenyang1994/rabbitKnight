package rabbit

import (
	"encoding/json"
	"log"
	"sync"
)

// KnightHub the knight message hub
type KnightHub struct {
	Lock            *sync.RWMutex
	clients         map[*KnightClient]bool
	broadcastStatus chan EventMsgForStatus
	broadcastQueue  chan EventMsgForQueue
	register        chan *KnightClient
	unregister      chan *KnightClient
}

func NewKnightHub() *KnightHub {
	lock := &sync.RWMutex{}
	hub := KnightHub{
		Lock:            lock,
		clients:         make(map[*KnightClient]bool),
		broadcastQueue:  make(chan EventMsgForQueue),
		broadcastStatus: make(chan EventMsgForStatus),
		unregister:      make(chan *KnightClient),
		register:        make(chan *KnightClient),
	}
	return &hub
}

// Run run the hub
func (hub *KnightHub) Run() {
	for {
		select {
		case client := <-hub.register:
			hub.clients[client] = true
		case client := <-hub.unregister:
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.send)
			}
		case message, ok := <-hub.broadcastStatus:
			for client := range hub.clients {
				if client.projectName == message.ProjectName || client.projectName == "ALL" {
					msg, err := json.Marshal(message)
					if err != nil {
						log.Fatal(err)
					}
					select {
					case client.send <- msg:
					default:
						close(client.send)
						delete(hub.clients, client)
					}
				} else {
					close(client.send)
					delete(hub.clients, client)
				}
			}

		case message, ok := <-hub.broadcastQueue:
			for client := range hub.clients {
				if client.projectName == message.ProjectName && client.queueName == message.QueueName {
					msg, err := json.Marshal(message)
					if err != nil {
						log.Fatal(err)
					}
					select {
					case client.send <- msg:
					default:
						close(client.send)
						delete(hub.clients, client)
					}
				} else {
					close(client.send)
					delete(hub.clients, client)
				}
			}

		}
	}

}

type KnightDoneHub struct {
	DoneMap map[string]chan<- struct{}
}

// NewKnightDoneHub ...
func NewKnightDoneHub(doneMap map[string]chan<- struct{}) *KnightDoneHub {
	donMap := KnightDoneHub{doneMap}
	return &donMap
}

// StopKnightByQueueName ...
func (doneHub *KnightDoneHub) StopKnightByQueueName(name string) {
	done := doneHub.DoneMap[name]
	log.Printf("wail close %s, close done channel", name)
	close(done)
}

func (doneHub *KnightDoneHub) StopAllKnight() {
	for _, done := range doneHub.DoneMap {
		close(done)
	}
}
