package rabbit

import (
	"log"
	"os"
	"rabbitKnight/utils"

	"github.com/streadway/amqp"
)

func setupChannel(amqpUrl string) (*amqp.Connection, *amqp.Channel, error) {
	url := os.Getenv(amqpUrl)

	conn, err := amqp.Dial(url)
	if err != nil {
		utils.LogOnError(err)
		return nil, nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		utils.LogOnError(err)
		return nil, nil, err
	}

	err = channel.Qos(1, 0, false)
	if err != nil {
		utils.LogOnError(err)
		return nil, nil, err
	}

	log.Printf("setup channel success!")

	return conn, channel, nil
}
