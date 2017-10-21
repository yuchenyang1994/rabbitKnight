package rabbit

import (
	"log"
	"rabbitKnight/utils"

	"github.com/streadway/amqp"
)

// NotifyResponse ...
type NotifyResponse bool

const (
	NotifySuccess = true
	NotifyFailure = false
)

// Message ....
type Message struct {
	queueConfig    QueueConfig    // configs
	amqpDelivery   *amqp.Delivery // message read from rabbitmq
	notifyResponse NotifyResponse
	notifyer       KnightNotifyer
}

// NewKnightMessage Create an Message
func NewKnightMessage(queueConfig QueueConfig, amqpDelivery *amqp.Delivery, notifyer KnightNotifyer) Message {
	msg := Message{queueConfig: queueConfig, amqpDelivery: amqpDelivery, notifyer: notifyer}
	return msg
}

// CurrentMessageRetries ...
func (m Message) CurrentMessageRetries() int {
	msg := m.amqpDelivery
	xDeathArray, ok := msg.Headers["x-death"].([]interface{})
	if !ok {
		m.Printf("x-death array case fail")
		return 0
	}

	if len(xDeathArray) <= 0 {
		return 0
	}

	for _, h := range xDeathArray {
		xDeathItem := h.(amqp.Table)

		if xDeathItem["reason"] == "rejected" {
			return int(xDeathItem["count"].(int64))
		}
	}

	return 0
}

// Notify .....
func (m *Message) Notify() *Message {
	msg := m.amqpDelivery
	ok := m.notifyer.NotifyConsumer(msg.Body)
	if ok {
		m.notifyResponse = NotifySuccess
	} else {
		m.notifyResponse = NotifyFailure
	}

	return m
}

// IsMaxRetry ....
func (m Message) IsMaxRetry() bool {
	retries := m.CurrentMessageRetries()
	maxRetries := m.queueConfig.RetryTimesWithDefault()
	return retries >= maxRetries
}

// IsNotifySuccess ...
func (m Message) IsNotifySuccess() bool {
	return m.notifyResponse == NotifySuccess
}

// Ack ...
func (m Message) Ack() error {
	m.Printf("acker: ack message")
	err := m.amqpDelivery.Ack(false)
	utils.LogOnError(err)
	return err
}

// Reject ...
func (m Message) Reject() error {
	m.Printf("acker: reject message")
	err := m.amqpDelivery.Reject(false)
	utils.LogOnError(err)
	return err
}

// Republish ....
func (m Message) Republish(out chan<- Message) error {
	m.Printf("acker: ERROR republish message")
	out <- m
	err := m.amqpDelivery.Ack(false)
	utils.LogOnError(err)
	return err
}

// CloneAndPublish ...
func (m Message) CloneAndPublish(channel *amqp.Channel) error {
	msg := m.amqpDelivery
	qc := m.queueConfig
	errMsg := cloneToPublishMsg(msg)
	err := channel.Publish(qc.ErrorExchangeName(), msg.RoutingKey, false, false, *errMsg)
	utils.LogOnError(err)
	return err
}

// Printf ...
func (m Message) Printf(v ...interface{}) {
	msg := m.amqpDelivery

	vv := []interface{}{}
	vv = append(vv, msg.MessageId, msg.RoutingKey)
	vv = append(vv, v[1:]...)

	log.Printf("[%s] [%s] "+v[0].(string), vv...)
}
