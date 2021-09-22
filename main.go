package main

import (
	"os"
	"time"
	"errors"
	"log"
	"bufio"
	"fmt"
	"net"
	"strings"
	"github.com/streadway/amqp"
)

func main(){


      listen,err := net.Listen("tcp",":80")
      if err != nil {
              fmt.Println("listen failed, err:", err)
              return
      }
      for {
              conn,err:=listen.Accept()
              if err != nil {
                      fmt.Println("accept failed, err:", err)
                      continue
              }
              fmt.Println("已连接到",conn.RemoteAddr().String())
              go process(conn)
      }
}
type Session struct {
	name            string
	logger          *log.Logger
	connection      *amqp.Connection
	channel         *amqp.Channel
	done            chan bool
	notifyConnClose chan *amqp.Error
	notifyChanClose chan *amqp.Error
	notifyConfirm   chan amqp.Confirmation
	isReady         bool
}

const (
	// When reconnecting to the server after connection failure
	reconnectDelay = 5 * time.Second

	// When setting up the channel after a channel exception
	reInitDelay = 2 * time.Second

	// When resending messages the server didn't confirm
	resendDelay = 5 * time.Second
)

var (
	errNotConnected  = errors.New("not connected to a server")
	errAlreadyClosed = errors.New("already closed: not connected to the server")
	errShutdown      = errors.New("session is shutting down")
)
func process(conn net.Conn){
        defer conn.Close() // 关闭连接
        hhy := user{"97815","韩浩源"}
        ywp := user{"97913","杨婉平"}
        userDB := make(map[string]string)
        userDB[hhy.code]=hhy.username
        userDB[ywp.code]=ywp.username
        userInput:=netRead(conn)
        CurrUsername, ok :=userDB [userInput]
        if ok{
                inputInfo := strings.Trim("您已成功连接 :" + CurrUsername, "\r\n")
                _, err := conn.Write([]byte(inputInfo)) // 发送数据
                if err != nil {
                        return
                }
        }else {
                inputInfo := strings.Trim("您已成功连接，陌生人", "\r\n")
                _, err := conn.Write([]byte(inputInfo)) // 发送数据
                if err != nil {
                        return
                }
        }
         addr := "amqp://admin:admin@1.117.233.210:5672/"
        rabbitmq_conn, err := amqp.Dial(addr)
        failOnError(err, "Failed to connect to RabbitMQ")
        defer rabbitmq_conn.Close()
        name := "job_queue"
        queue := New(name, addr)
        // Attempt to push a message every 2 seconds
        for {
		userInput:=netRead(conn)
  		message := []byte(userInput)
	       if err := queue.Push(message); err != nil {
                       fmt.Printf("Push failed: %s\n", err)
                } else {
                        fmt.Println("Push succeeded!")
                }
 
	}

		

}

func New(name string, addr string) *Session {
	session := Session{
		logger: log.New(os.Stdout, "", log.LstdFlags),
		name:   name,
		done:   make(chan bool),
	}
	go session.handleReconnect(addr)
	return &session
}

// handleReconnect will wait for a connection error on
// notifyConnClose, and then continuously attempt to reconnect.
func (session *Session) handleReconnect(addr string) {
	for {
		session.isReady = false
		log.Println("Attempting to connect")

		conn, err := session.connect(addr)

		if err != nil {
			log.Println("Failed to connect. Retrying...")

			select {
			case <-session.done:
				return
			case <-time.After(reconnectDelay):
			}
			continue
		}

		if done := session.handleReInit(conn); done {
			break
		}
	}
}

// connect will create a new AMQP connection
func (session *Session) connect(addr string) (*amqp.Connection, error) {
	conn, err := amqp.Dial(addr)

	if err != nil {
		return nil, err
	}

	session.changeConnection(conn)
	log.Println("Connected!")
	return conn, nil
}

// handleReconnect will wait for a channel error
// and then continuously attempt to re-initialize both channels
func (session *Session) handleReInit(conn *amqp.Connection) bool {
	for {
		session.isReady = false

		err := session.init(conn)

		if err != nil {
			log.Println("Failed to initialize channel. Retrying...")

			select {
			case <-session.done:
				return true
			case <-time.After(reInitDelay):
			}
			continue
		}

		select {
		case <-session.done:
			return true
		case <-session.notifyConnClose:
			log.Println("Connection closed. Reconnecting...")
			return false
		case <-session.notifyChanClose:
			log.Println("Channel closed. Re-running init...")
		}
	}
}

// init will initialize channel & declare queue
func (session *Session) init(conn *amqp.Connection) error {
	ch, err := conn.Channel()

	if err != nil {
		return err
	}

	err = ch.Confirm(false)

	if err != nil {
		return err
	}
	_, err = ch.QueueDeclare(
		session.name,
		false, // Durable
		false, // Delete when unused
		false, // Exclusive
		false, // No-wait
		nil,   // Arguments
	)

	if err != nil {
		return err
	}

	session.changeChannel(ch)
	session.isReady = true
	log.Println("Setup!")

	return nil
}

// changeConnection takes a new connection to the queue,
// and updates the close listener to reflect this.
func (session *Session) changeConnection(connection *amqp.Connection) {
	session.connection = connection
	session.notifyConnClose = make(chan *amqp.Error)
	session.connection.NotifyClose(session.notifyConnClose)
}

// changeChannel takes a new channel to the queue,
// and updates the channel listeners to reflect this.
func (session *Session) changeChannel(channel *amqp.Channel) {
	session.channel = channel
	session.notifyChanClose = make(chan *amqp.Error)
	session.notifyConfirm = make(chan amqp.Confirmation, 1)
	session.channel.NotifyClose(session.notifyChanClose)
	session.channel.NotifyPublish(session.notifyConfirm)
}

// Push will push data onto the queue, and wait for a confirm.
// If no confirms are received until within the resendTimeout,
// it continuously re-sends messages until a confirm is received.
// This will block until the server sends a confirm. Errors are
// only returned if the push action itself fails, see UnsafePush.
func (session *Session) Push(data []byte) error {
	if !session.isReady {
		return errors.New("failed to push push: not connected")
	}
	for {
		err := session.UnsafePush(data)
		if err != nil {
			session.logger.Println("Push failed. Retrying...")
			select {
			case <-session.done:
				return errShutdown
			case <-time.After(resendDelay):
			}
			continue
		}
		select {
		case confirm := <-session.notifyConfirm:
			if confirm.Ack {
				session.logger.Println("Push confirmed!")
				return nil
			}
		case <-time.After(resendDelay):
		}
		session.logger.Println("Push didn't confirm. Retrying...")
	}
}

// UnsafePush will push to the queue without checking for
// confirmation. It returns an error if it fails to connect.
// No guarantees are provided for whether the server will
// recieve the message.
func (session *Session) UnsafePush(data []byte) error {
	if !session.isReady {
		return errNotConnected
	}
	return session.channel.Publish(
		"",           // Exchange
		session.name, // Routing key
		false,        // Mandatory
		false,        // Immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        data,
		},
	)
}

// Stream will continuously put queue items on the channel.
// It is required to call delivery.Ack when it has been
// successfully processed, or delivery.Nack when it fails.
// Ignoring this will cause data to build up on the server.
func (session *Session) Stream() (<-chan amqp.Delivery, error) {
	if !session.isReady {
		return nil, errNotConnected
	}
	return session.channel.Consume(
		session.name,
		"",    // Consumer
		false, // Auto-Ack
		false, // Exclusive
		false, // No-local
		false, // No-Wait
		nil,   // Args
	)
}

// Close will cleanly shutdown the channel and connection.
func (session *Session) Close() error {
	if !session.isReady {
		return errAlreadyClosed
	}
	err := session.channel.Close()
	if err != nil {
		return err
	}
	err = session.connection.Close()
	if err != nil {
		return err
	}
	close(session.done)
	session.isReady = false
	return nil
}

func netRead(conn net.Conn) string {
	reader := bufio.NewReader(conn)
	var buf [128]byte
	n, err := reader.Read(buf[:]) // 读取数据
	if err != nil {
		fmt.Println("read from client failed, err:", err)
	}
	recvStr := string(buf[:n-2])
	return  recvStr
}
func failOnError(err error, msg string) {
    if err != nil {
        log.Fatalf("%s: %s", msg, err)
    }
}

