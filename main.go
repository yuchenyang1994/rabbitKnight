package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"rabbitKnight/rabbit"
	"rabbitKnight/utils"
	"syscall"
)

var (
	amqpConfig     = flag.String("mq", "127.0.0.1:5671", "rabbtmq addr")
	serverPort     = flag.String("port", ":8080", "server port")
	configFilename = flag.String("queue_config", "./config/queue_config.yaml", "the queues config file")
	logFileName    = flag.String("log", "", "logging file, default STDOUT")
)

func main() {
	flag.Parse()
	hub := rabbit.NewKnightHub()
	go hub.Run()
	if *logFileName != "" {
		f, err := os.OpenFile(*logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		utils.PanicOnError(err)
		defer f.Close()

		log.SetOutput(f)
	}
	configManager := rabbit.NewKnightConfigManager(*configFilename)
	allQueueConfigs := configManager.LoadQueuesConfig()
	man := rabbit.NewRabbitKnightMan(allQueueConfigs, *amqpConfig, hub)
	done := make(chan struct{}, 1)
	man.RunKnight(done)
	handleSignal(done)
}

func handleSignal(done chan<- struct{}) {
	chan_sigs := make(chan os.Signal, 1)
	signal.Notify(chan_sigs, syscall.SIGQUIT)

	go func() {
		sig := <-chan_sigs

		if sig != nil {
			log.Printf("received a signal %v, close done channel", sig)
			close(done)
		}
	}()
}
