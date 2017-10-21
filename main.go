package main

import (
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"rabbitKnight/rabbit"
	"rabbitKnight/utils"
	"syscall"
)

var (
	amqpConfig     = flag.String("mq", "amqp://guest:guest@172.17.0.2:5672", "rabbtmq URL")
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
	doneHub := RunQueueKnight(hub)
	handleSignal(doneHub)
	// Server
	http.HandleFunc("/hello", HelloServer)
	err := http.ListenAndServe(*serverPort, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func HelloServer(w http.ResponseWriter, req *http.Request) {
	io.WriteString(w, "hello, world!\n")
}

func RunQueueKnight(hub *rabbit.KnightHub) *rabbit.KnightDoneHub {
	configManager := rabbit.NewKnightConfigManager(*configFilename)
	allQueueConfigs := configManager.LoadQueuesConfig()
	doneMap := make(map[string]chan<- struct{})
	for _, queueConfig := range allQueueConfigs {
		done := make(chan struct{}, 1)
		doneMap[queueConfig.QueueName] = done
		man := rabbit.NewRabbitKnightMan(queueConfig, *amqpConfig, hub)
		go man.RunKnight(done)
	}
	doneHub := rabbit.NewKnightDoneHub(doneMap)
	return doneHub
}

func handleSignal(doneHub *rabbit.KnightDoneHub) {
	chan_sigs := make(chan os.Signal, 1)
	signal.Notify(chan_sigs, syscall.SIGQUIT)
	go func() {
		sig := <-chan_sigs

		if sig != nil {
			log.Printf("received a signal %v, close done channel", sig)
			doneHub.StopAllKnight()
		}
	}()
}
