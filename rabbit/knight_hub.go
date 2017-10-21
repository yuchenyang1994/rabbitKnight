package rabbit

import (
	"encoding/json"
	"log"
	"rabbitKnight/utils"
	"sync"
)

// KnightHub the knight message hub
type KnightHub struct {
	Lock       *sync.RWMutex
	ErrorMsgs  map[string][]EventMsgForJSON
	clients    map[*KnightClient]bool
	broadcast  chan []byte
	register   chan *KnightClient
	unregister chan *KnightClient
}

func NewKnightHub() *KnightHub {
	lock := &sync.RWMutex{}
	hub := KnightHub{
		Lock:       lock,
		ErrorMsgs:  make(map[string][]EventMsgForJSON),
		clients:    make(map[*KnightClient]bool),
		broadcast:  make(chan []byte),
		unregister: make(chan *KnightClient),
		register:   make(chan *KnightClient),
	}
	return &hub
}

// Run run the hub
func (hub *KnightHub) Run() {
	for {
		select {
		case client := <-hub.register:
			hub.clients[client] = true
			errorMsgs, err := json.Marshal(hub.ErrorMsgs)
			if err != nil {
				utils.LogOnError(err)
			}
			hub.broadcast <- errorMsgs

		case client := <-hub.unregister:
			if _, ok := hub.clients[client]; ok {
				delete(hub.clients, client)
				close(client.send)
			}
		case message := <-hub.broadcast:
			log.Println(string(message))
			for client := range hub.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(hub.clients, client)
				}
			}
		}
	}

}

// SetErrorMsgs ...
func (hub *KnightHub) SetErrorMsgs(projectName string, errorEvent EventMsgForJSON) {
	hub.Lock.Lock()
	defer hub.Lock.Unlock()
	events := hub.ErrorMsgs[projectName]
	events = append(events, errorEvent)
	hub.ErrorMsgs[projectName] = events
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
