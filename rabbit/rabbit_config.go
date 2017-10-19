package rabbit

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"rabbitKnight/utils"

	"github.com/streadway/amqp"
	yaml "gopkg.in/yaml.v2"
)

type ProjectsConfig struct {
	Projects []ProjectConfig `json:"projects"`
}

// ProjectConfig list queue configs 批量配置时的配置文件
type ProjectConfig struct {
	Name                string              `yaml:"name"`           // 项目名称
	QueuesDefaultConfig QueuesDefaultConfig `yaml:"queues_default"` // 默认配置
	Queues              []QueueConfig       `yaml:"queues"`         // 队列配置
}

// QueuesDefaultConfig 队列默认配置
type QueuesDefaultConfig struct {
	NotifyBase      string `yaml:"notifyBase"` // notyfy Host
	NotifyMethod    string `yaml:"notifyMethod"`
	NotifyTimeout   int    `yaml:"notifyTimeout"`   // 全局过期时间
	RetryTimes      int    `yaml:"retryTimes"`      // 重试时间
	RetryDuration   int    `yaml:"retryDuration"`   // 重试次数
	BindingExchange string `yaml:"bindingExchange"` // 绑定RabbbitMqExchange
}

// QueueConfig 单独队列设置
type QueueConfig struct {
	QueueName       string   `yaml:"queueName"`
	NotifyMethod    string   `yaml:"notifyMethod"`
	RpcFunc         string   `yaml:"rpcFunc"`
	RoutingKey      []string `yaml:"routingKey"`
	NotifyPath      string   `yaml:"notifyPath"`
	NotifyTimeout   int      `yaml:"notifyTimeout"`
	RetryTimes      int      `yaml:"retryTimes"`
	RetryDuration   int      `yaml:"retryDuration"`
	BindingExchange string   `yaml:"bindingExchange"`
	project         *ProjectConfig
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
	return qc.BindingExchange
}

func (qc QueueConfig) GetNotifyMethod() string {
	if qc.NotifyMethod == "" {
		return qc.project.QueuesDefaultConfig.NotifyMethod
	} else {
		return qc.NotifyMethod
	}
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

// LoadQueuesConfig ....
func LoadQueuesConfig(configFileName string, allQueues []*QueueConfig) []*QueueConfig {
	configFile, err := ioutil.ReadFile(configFileName)
	utils.PanicOnError(err)

	projectsConfig := ProjectsConfig{}
	err = yaml.Unmarshal(configFile, &projectsConfig)
	utils.PanicOnError(err)
	log.Printf("find config: %v", projectsConfig)

	projects := projectsConfig.Projects
	for i, project := range projects {
		log.Printf("find project: %s", project.Name)

		queues := projects[i].Queues
		for j, queue := range queues {
			log.Printf("find queue: %v", queue)

			queues[j].project = &projects[i]
			allQueues = append(allQueues, &queues[j])
		}
	}

	return allQueues
}
