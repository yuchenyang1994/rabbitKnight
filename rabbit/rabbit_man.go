package rabbit

import (
	"fmt"
	"log"
	"rabbitKnight/utils"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/streadway/amqp"
)

const (
	// chan
	ChannelBufferLength = 100

	//worker number
	ReceiverNum = 5
	AckerNum    = 10
	ResenderNum = 5

	// http tune
	HttpMaxIdleConns        = 500 // default 100 in net/http
	HttpMaxIdleConnsPerHost = 500 // default 2 in net/http
	HttpIdleConnTimeout     = 30  // default 90 in net/http
)

type RabbitKnightMan struct {
	queues []*QueueConfig
}

// receiveMessage ...
func (man *RabbitKnightMan) receiveMessage(done <-chan struct{}) <-chan Message {
	out := make(chan Message, ChannelBufferLength)
	var wg sync.WaitGroup
	receiver := func(qc QueueConfig) {
		defer wg.Done()
	RECONNECT:
		for {
			_, channel, err := setupChannel()
			if err != nil {
				utils.PanicOnError(err)
			}
			msgs, err := channel.Consume(
				qc.WorkerQueueName(), // queue
				"",                   // consumer
				false,                // auto-ack
				false,                // exclusive
				false,                // no-local
				false,                // no-wait
				nil,                  // args
			)
			utils.PanicOnError(err)
			for {
				select {
				case msg, ok := <-msgs:
					if !ok {
						log.Printf("receiver: channel is closed, maybe lost connection")
						time.Sleep(5 * time.Second)
						continue RECONNECT
					}
					msg.MessageId = fmt.Sprintf("%s", uuid.NewV4())
					client := newHttpClient(HttpMaxIdleConns, HttpMaxIdleConnsPerHost, HttpIdleConnTimeout)
					client.Timeout = time.Duration(qc.NotifyTimeoutWithDefault()) * time.Second
					notifyer := NewApiNotiFyer(qc, client)
					message := NewKnightMessage(qc, &msg, &notifyer)
					out <- message
					message.Printf("receiver: received msg")
				case <-done:
					log.Printf("receiver: received a done signal")
					return
				}
			}
		}
	}
	for _, queue := range man.queues {
		wg.Add(ReceiverNum)
		for i := 0; i < ReceiverNum; i++ {
			go receiver(*queue)
		}
	}
	go func() {
		wg.Wait()
		log.Printf("all receiver is done, closing channel")
		close(out)
	}()
	return out
}

// workMessage work work
func (man *RabbitKnightMan) workMessage(in <-chan Message) <-chan Message {
	var wg sync.WaitGroup
	out := make(chan Message, ChannelBufferLength)
	work := func(m Message, o chan<- Message) {
		m.Printf("worker: received a msg, body: %s", string(m.amqpDelivery.Body))
		defer wg.Done()
		m.Notify()
		o <- m
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for message := range in {
			wg.Add(1)
			go work(message, out)
		}

	}()
	go func() {
		wg.Wait()
		log.Printf("all worker is done", "closing channel")
		close(out)
	}()
	return out
}

// ackMessage ack message
func (man *RabbitKnightMan) ackMessage(in <-chan Message) <-chan Message {
	out := make(chan Message)
	var wg sync.WaitGroup
	acker := func() {
		defer wg.Done()

		for m := range in {
			m.Printf("acker: received a msg")

			if m.IsNotifySuccess() {
				m.Ack()
			} else if m.IsMaxRetry() {
				m.Republish(out)
			} else {
				m.Reject()
			}
		}
	}
	for i := 0; i < AckerNum; i++ {
		wg.Add(1)
		go acker()
	}

	go func() {
		wg.Wait()
		log.Printf("all acker is done, close out")
		close(out)
	}()

	return out
}

// resendMessage
func (man *RabbitKnightMan) resendMessage(in <-chan Message) <-chan Message {
	out := make(chan Message)

	var wg sync.WaitGroup

	resender := func() {
		defer wg.Done()

	RECONNECT:
		for {
			conn, channel, err := setupChannel()
			if err != nil {
				utils.PanicOnError(err)
			}

			for m := range in {
				err := m.CloneAndPublish(channel)
				if err == amqp.ErrClosed {
					time.Sleep(5 * time.Second)
					continue RECONNECT
				}
			}

			// normally quit , we quit too
			conn.Close()
			break
		}
	}

	for i := 0; i < ResenderNum; i++ {
		wg.Add(1)
		go resender()
	}

	go func() {
		wg.Wait()
		log.Printf("all resender is done, close out")
		close(out)
	}()

	return out
}

// RunKnight run the watch
func (man *RabbitKnightMan) RunKnight(done <-chan struct{}) {
	_, channel, err := setupChannel()
	if err != nil {
		utils.PanicOnError(err)
	}
	for _, queue := range man.queues {
		log.Printf("allQueues: queue config: %v", queue)
		queue.DeclareExchange(channel)
		queue.DeclareQueue(channel)
	}
	<-man.resendMessage(man.ackMessage(man.workMessage(man.receiveMessage(done))))
}
