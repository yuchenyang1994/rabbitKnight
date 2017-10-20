package rabbit

import (
	"encoding/json"
	"log"
	"rabbitKnight/utils"
)

// HubErrorCache ...
type HubErrorCache struct {
	QueueName string   `json:"queueName"`
	ErrorMsgs []string `json:"messages"`
}

// KnightHub the knight message hub
type KnightHub struct {
	clients    map[*KnightClient]bool
	broadcast  chan []byte
	register   chan *KnightClient
	unregister chan *KnightClient
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
			log.Println(message)
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
