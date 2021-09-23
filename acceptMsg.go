package main

import (
	"fmt"
	"github.com/streadway/amqp"
	"net"
)

func acceptMessage(conn net.Conn,usr user,rabbitmq_conn *amqp.Connection,addr string ) {
	defer conn.Close() // 关闭连接
	defer rabbitmq_conn.Close()
	name := usr.username
	queue := New(name, addr)
	// Attempt to push a message every 2 seconds
	for {
		fmt.Println("在读取数据")
		userInput,err:=netRead(conn)
		if err != nil {
			fmt.Printf("客户端退出")
			return
		}
		message := []byte(userInput)
		//fmt.Println()
		if err := queue.Push(message); err != nil {
			fmt.Printf("Push failed: %s\n", err)
			fmt.Printf("客户端退出")
			return
		} else {
			fmt.Println("Push succeeded!")
		}
	}
}