package rabbit

import (
	"fmt"
	"log"
	"strings"

	"rabbitKnight/utils"

	"github.com/streadway/amqp"
)

// ProjectConfig list queue configs 批量配置时的配置文件
type ProjectConfig struct {
	Name                string              `json:"name"`           // 项目名称
	QueuesDefaultConfig QueuesDefaultConfig `json:"queues_default"` // 默认配置
	Queues              []QueueConfig       `json:"queues"`         // 队列配置
}

// QueuesDefaultConfig 队列默认配置
type QueuesDefaultConfig struct {
	NotifyBase      string `json:"notify_base"`      // notyfy Host
	NotifyTimeout   int    `json:"notify_timeout"`   // 全局过期时间
	RetryTimes      int    `json:"retry_times"`      // 重试时间
	RetryDuration   int    `json:"retry_duration"`   // 重试次数
	BindingExchange string `json:"binding_exchange"` // 绑定RabbbitMqExchange
}

// QueueConfig 单独队列设置
type QueueConfig struct {
	QueueName       string   `json:"queue_name"`
	RoutingKey      []string `json:"routing_key"`
	NotifyPath      string   `json:"notify_path"`
	NotifyTimeout   int      `json:"notify_timeout"`
	RetryTimes      int      `json:"retry_times"`
	RetryDuration   int      `json:"retry_duration"`
	BindingExchange string   `json:"binding_exchange"`

	project *ProjectConfig
}

func (qc QueueConfig) WorkerQueueName() string {
	return qc.QueueName
}
func (qc QueueConfig) RetryQueueName() string {
	return fmt.Sprintf("%s-retry", qc.QueueName)
}
func (qc QueueConfig) ErrorQueueName() string {
	return fmt.Sprintf("%s-error", qc.QueueName)
}
func (qc QueueConfig) RetryExchangeName() string {
	return fmt.Sprintf("%s-retry", qc.QueueName)
}
func (qc QueueConfig) RequeueExchangeName() string {
	return fmt.Sprintf("%s-retry-requeue", qc.QueueName)
}
func (qc QueueConfig) ErrorExchangeName() string {
	return fmt.Sprintf("%s-error", qc.QueueName)
}
func (qc QueueConfig) WorkerExchangeName() string {
	if qc.BindingExchange == "" {
		return qc.project.QueuesDefaultConfig.BindingExchange
	}
	return qc.BindingExchange
}

func (qc QueueConfig) NotifyUrl() string {
	if strings.HasPrefix(qc.NotifyPath, "http://") || strings.HasPrefix(qc.NotifyPath, "https://") {
		return qc.NotifyPath
	}
	return fmt.Sprintf("%s%s", qc.project.QueuesDefaultConfig.NotifyBase, qc.NotifyPath)
}

func (qc QueueConfig) NotifyTimeoutWithDefault() int {
	if qc.NotifyTimeout == 0 {
		return qc.project.QueuesDefaultConfig.NotifyTimeout
	}
	return qc.NotifyTimeout
}

func (qc QueueConfig) RetryTimesWithDefault() int {
	if qc.RetryTimes == 0 {
		return qc.project.QueuesDefaultConfig.RetryTimes
	}
	return qc.RetryTimes
}

func (qc QueueConfig) RetryDurationWithDefault() int {
	if qc.RetryDuration == 0 {
		return qc.project.QueuesDefaultConfig.RetryDuration
	}
	return qc.RetryDuration
}

func (qc QueueConfig) DeclareExchange(channel *amqp.Channel) {
	exchanges := []string{
		qc.WorkerExchangeName(),
		qc.RetryExchangeName(),
		qc.ErrorExchangeName(),
		qc.RequeueExchangeName(),
	}

	for _, e := range exchanges {
		log.Printf("declaring exchange: %s\n", e)

		err := channel.ExchangeDeclare(e, "topic", true, false, false, false, nil)
		utils.PanicOnError(err)
	}
}

func (qc QueueConfig) DeclareQueue(channel *amqp.Channel) {
	var err error

	// 定义重试队列
	log.Printf("declaring retry queue: %s\n", qc.RetryQueueName())
	retryQueueOptions := map[string]interface{}{
		"x-dead-letter-exchange": qc.RequeueExchangeName(),
		"x-message-ttl":          int32(qc.RetryDurationWithDefault() * 1000),
	}

	_, err = channel.QueueDeclare(qc.RetryQueueName(), true, false, false, false, retryQueueOptions)
	utils.PanicOnError(err)
	err = channel.QueueBind(qc.RetryQueueName(), "#", qc.RetryExchangeName(), false, nil)
	utils.PanicOnError(err)

	// 定义错误队列
	log.Printf("declaring error queue: %s\n", qc.ErrorQueueName())

	_, err = channel.QueueDeclare(qc.ErrorQueueName(), true, false, false, false, nil)
	utils.PanicOnError(err)
	err = channel.QueueBind(qc.ErrorQueueName(), "#", qc.ErrorExchangeName(), false, nil)
	utils.PanicOnError(err)

	// 定义工作队列
	log.Printf("declaring worker queue: %s\n", qc.WorkerQueueName())

	workerQueueOptions := map[string]interface{}{
		"x-dead-letter-exchange": qc.RetryExchangeName(),
	}
	_, err = channel.QueueDeclare(qc.WorkerQueueName(), true, false, false, false, workerQueueOptions)
	utils.PanicOnError(err)

	for _, key := range qc.RoutingKey {
		err = channel.QueueBind(qc.WorkerQueueName(), key, qc.WorkerExchangeName(), false, nil)
		utils.PanicOnError(err)
	}

	// 最后，绑定工作队列 和 requeue Exchange
	err = channel.QueueBind(qc.WorkerQueueName(), "#", qc.RequeueExchangeName(), false, nil)
	utils.PanicOnError(err)
}
