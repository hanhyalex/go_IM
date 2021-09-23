package main

import (
	"github.com/streadway/amqp"
	"net"
	"fmt"
	"log"
)

func writeMessage(conn net.Conn,other user,rabbitmq_conn *amqp.Connection,addr string)  {
	//defer conn.Close() // 关闭连接
	//defer rabbitmq_conn.Close()
	channel ,err:= rabbitmq_conn.Channel()
	if err != nil {
		return
	}
	//queue:= channel.QueueBind(
	//	"杨婉平", // TODO name of the queue
	//	"test-key",        // bindingKey
	//	"direct",   // sourceExchange
	//	false,      // noWait
	//	nil,        // arguments
	//)
	deliveries, err := channel.Consume(
		other.username, // name
		"simple-consumer",      // consumerTag,
		false,      // noAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		fmt.Println("Queue Consume: %s", err)
		return
	}
	for {
		for d := range deliveries {
			log.Printf(
				"got %dB delivery: [%v] %q",
				len(d.Body),
				d.DeliveryTag,
				d.Body,
			)
			inputInfo :=other.username+": "+string(d.Body)+"\r\n"
			_, err := conn.Write([]byte(inputInfo)) // 发送数据

			if err != nil {
				fmt.Println(err)
			}
			d.Ack(false)
		}
	}

}
