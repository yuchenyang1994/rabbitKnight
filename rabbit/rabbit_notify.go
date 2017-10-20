package rabbit

import (
	"log"
	"net/http"
	"time"

	"bytes"

	"github.com/streadway/amqp"
	"github.com/ybbus/jsonrpc"
)

// KnightNotifyer NotyfiConsumer
type KnightNotifyer interface {
	NotifyConsumer(body []byte) bool
}

// CloneToPublishMsg ...
func cloneToPublishMsg(msg *amqp.Delivery) *amqp.Publishing {
	newMsg := amqp.Publishing{
		Headers:         msg.Headers,
		ContentType:     msg.ContentType,
		ContentEncoding: msg.ContentEncoding,
		DeliveryMode:    msg.DeliveryMode,
		Priority:        msg.Priority,
		CorrelationId:   msg.CorrelationId,
		ReplyTo:         msg.ReplyTo,
		Expiration:      msg.Expiration,
		MessageId:       msg.MessageId,
		Timestamp:       msg.Timestamp,
		Type:            msg.Type,
		UserId:          msg.UserId,
		AppId:           msg.AppId,

		Body: msg.Body,
	}
	return &newMsg
}

func newHttpClient(maxIdleConns, maxIdleConnsPerHost, idleConnTimeout int) *http.Client {
	tr := &http.Transport{
		MaxIdleConns:        maxIdleConns,
		MaxIdleConnsPerHost: maxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(idleConnTimeout) * time.Second,
	}

	client := &http.Client{
		Transport: tr,
	}
	return client
}

func NewApiNotiFyer(queueConfig QueueConfig, client *http.Client) ApiNotifyer {
	notifyer := ApiNotifyer{client, queueConfig}
	return notifyer
}

// ApiNotifyer Notify user with api
type ApiNotifyer struct {
	httpClient  *http.Client // http client
	queueConfig QueueConfig
}

// NotifyConsumer ...
func (notifyer *ApiNotifyer) NotifyConsumer(body []byte) bool {
	qc := notifyer.queueConfig
	req, err := http.NewRequest("POST", qc.NotifyUrl(), bytes.NewReader(body))
	if err != nil {
		log.Printf("notify url create fail")
	}
	req.Header.Set("Content-Type", "application/json")
	response, err := notifyer.httpClient.Do(req)
	if err != nil {
		log.Panicf("notifu url %s fail:%s", qc.NotifyUrl(), err)
		return false
	}
	defer response.Body.Close()
	if response.StatusCode == 200 {
		return true
	}
	return false
}

// NewApiNotiFyer crate Jsonrpc Notify
func NewJSONRPCNotifyer(queueConfig QueueConfig, client *http.Client) JsonRpcNotifyer {
	n := JsonRpcNotifyer{httpClient: client, queueConfig: queueConfig}
	return n
}

// JsonRpcNotifyer ...
type JsonRpcNotifyer struct {
	httpClient  *http.Client // http client
	queueConfig QueueConfig
}

// NotifyConsumer JsonRpcNotifyer
func (notifyer *JsonRpcNotifyer) NotifyConsumer(body []byte) bool {
	rpcClient := jsonrpc.NewRPCClient(notifyer.queueConfig.NotifyUrl())
	rpcClient.SetHTTPClient(notifyer.httpClient)
	err := rpcClient.Notification(notifyer.queueConfig.RpcFunc, body)
	if err == nil {
		return true
	}
	return false
}
