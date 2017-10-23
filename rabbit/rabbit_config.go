package rabbit

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"rabbitKnight/utils"

	"sync"

	"os"

	"encoding/json"

	"github.com/streadway/amqp"
	yaml "gopkg.in/yaml.v2"
)

var (
	knightManager *KnightConfigManager
	once          sync.Once
)

// KnightConfigManager manager for mq configs
type KnightConfigManager struct {
	ConfigFileName string
	Lock           *sync.RWMutex
	Configs        *ProjectsConfig
	AmqpConfig     string
}

type ProjectsConfig struct {
	Projects map[string]ProjectConfig `yaml:"projects" json:"projects"`
}

// ProjectConfig list queue configs 批量配置时的配置文件
type ProjectConfig struct {
	Name                string                 `yaml:"name" json:"name"`                  // 项目名称
	QueuesDefaultConfig QueuesDefaultConfig    `yaml:"queuesDefault" json:"queueDefault"` // 默认配置
	Queues              map[string]QueueConfig `yaml:"queues" json:"queues"`              // 队列配置
}

// QueuesDefaultConfig 队列默认配置
type QueuesDefaultConfig struct {
	NotifyBase      string `yaml:"notifyBase" json:"notifyBase"`       // notyfy Host NotifyMethod    string `yaml:"notifyMethod" json:"notifyMethod"`
	NotifyTimeout   int    `yaml:"notifyTimeout" json:"notifyTimeout"` // 全局过期时间
	NotifyMethod    string `yaml:"notifyMethod" json:"notifyMethod"`
	RetryTimes      int    `yaml:"retryTimes" json:"retryTimes"`           // 重试时间
	RetryDuration   int    `yaml:"retryDuration" json:"retryDuration"`     // 重试次数
	BindingExchange string `yaml:"bindingExchange" json:"bindingExchange"` // 绑定RabbbitMqExchange
}

// QueueConfig 单独队列设置
type QueueConfig struct {
	QueueName       string   `yaml:"queueName" json:"queueName"`
	NotifyMethod    string   `yaml:"notifyMethod" json:"notifyMethod"`
	RpcFunc         string   `yaml:"rpcFunc" json:"rpcFunc"`
	RoutingKey      []string `yaml:"routingKey" json:"routingKey"`
	NotifyPath      string   `yaml:"notifyPath" json:"notifyPath"`
	NotifyTimeout   int      `yaml:"notifyTimeout" json:"notifyTimeout"`
	RetryTimes      int      `yaml:"retryTimes" json:"retryTimes"`
	RetryDuration   int      `yaml:"retryDuration" json:"retryDuration"`
	BindingExchange string   `yaml:"bindingExchange" json:"bindingExchange"`
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
	if qc.BindingExchange == "" {
		return qc.project.QueuesDefaultConfig.BindingExchange
	}
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
		fmt.Println(e)
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

// NewKnightConfigManager ...
func NewKnightConfigManager(configFileName string, amqp string) *KnightConfigManager {
	once.Do(func() {
		lock := sync.RWMutex{}
		knightManager = &KnightConfigManager{ConfigFileName: configFileName, Lock: &lock, AmqpConfig: amqp}
	})
	return knightManager
}

// LoadQueuesConfig ....
func (manager *KnightConfigManager) LoadQueuesConfig() []*QueueConfig {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	allQueues := []*QueueConfig{}
	configFile, err := ioutil.ReadFile(manager.ConfigFileName)
	utils.PanicOnError(err)
	projectsConfig := ProjectsConfig{}
	err = yaml.Unmarshal(configFile, &projectsConfig)
	utils.PanicOnError(err)
	projects := projectsConfig.Projects
	for i, project := range projects {
		log.Printf("find project: %s", i)
		project.Name = i
		queues := project.Queues
		for j, queue := range queues {
			log.Printf("find queue: %v", j)
			queue.project = &project
			queue.project.Name = i
			allQueues = append(allQueues, &queue)
		}
	}
	manager.Configs = &projectsConfig

	return allQueues
}

// LoadQueuesForJSON ...
func (manager *KnightConfigManager) LoadQueuesForJSON(jsonBody []byte) []*QueueConfig {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	allQueues := []*QueueConfig{}
	projectsConfig := ProjectsConfig{}
	projects := projectsConfig.Projects
	json.Unmarshal(jsonBody, &projectsConfig)
	for i, project := range projects {
		project.Name = i
		log.Printf("find project: %s", i)
		queues := project.Queues
		for j, queue := range queues {
			log.Printf("find queue: %v", j)
			queue.project = &project
			allQueues = append(allQueues, &queue)
		}
	}
	manager.Configs = &projectsConfig

	return allQueues
}

func (manager *KnightConfigManager) GetQueueConfig(queueName string, projectName string) *QueueConfig {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	project := manager.Configs.Projects[projectName]
	queueConfig := project.Queues[queueName]
	queueConfig.project = &project
	return &queueConfig
}

func (manager *KnightConfigManager) SetProjectForName(projectName string, queueConfig QueueConfig) *QueueConfig {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	project := manager.Configs.Projects[projectName]
	queueConfig.project = &project
	return &queueConfig
}

// SaveQueuesConfig ...
func (manager *KnightConfigManager) SaveQueuesConfig(config QueueConfig, projectName string, queueName string) {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	project := manager.Configs.Projects[projectName]
	project.Queues[queueName] = config
	manager.Configs.Projects[projectName] = project
	configs, err := yaml.Marshal(manager.Configs.Projects)
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.OpenFile(manager.ConfigFileName, os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.WriteString(string(configs))
	if err != nil {
		log.Fatal(err)
	}
	file.Sync()
}

func (manager *KnightConfigManager) SaveAllQueues(jsonBody []byte) {
	manager.Lock.Lock()
	defer manager.Lock.Unlock()
	projectsConfig := ProjectsConfig{}
	projects := projectsConfig.Projects
	json.Unmarshal(jsonBody, &projectsConfig)
	for projectName, project := range projects {
		project.Name = projectName
		manager.Configs.Projects[projectName] = project
	}
	configs, err := yaml.Marshal(manager.Configs.Projects)
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.OpenFile(manager.ConfigFileName, os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.WriteString(string(configs))
	if err != nil {
		log.Fatal(err)
	}
	file.Sync()
}
