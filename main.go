package main

import (
        //"time"
        "fmt"
        "log"
        "net/http"
        //"encoding/binary"
        "github.com/gorilla/websocket"
)

const MESSAGE_TYPE_CREATE_RECEIVE_STREAM = 0
const MESSAGE_TYPE_STREAM_DATA = 1
const MESSAGE_TYPE_STREAM_END = 2
const MESSAGE_TYPE_TERMINATE_SEND_STREAM = 3
const MESSAGE_TYPE_STREAM_REQUEST_DATA = 4
const MESSAGE_TYPE_CONTROL_MESSAGE = 5

type Producer interface {
        OnData(callback func([]byte))
        Request(numElements uint8)
}

func CreateConnection() Connection {
        return Connection{
                receiveStreams: make(map[byte]*ReceiveStream),
        }
}


type Connection struct {
        send func([]byte)
        receiveStreams map[byte]*ReceiveStream
        streamCallback func(*ReceiveStream)
}

func (c *Connection) sendControlMessage(message []byte) {
        var reqMsg [2]byte
        reqMsg[0] = MESSAGE_TYPE_CONTROL_MESSAGE
        reqMsg[1] = 22
        c.send(reqMsg[:])
}

func (c *Connection) HandleMessage(message []byte) {
        messageType := message[0]

        if messageType == MESSAGE_TYPE_CONTROL_MESSAGE {
                log.Println("Control message")
        } else {
                streamId := message[1]
                switch messageType {
                case MESSAGE_TYPE_CREATE_RECEIVE_STREAM:
                        log.Printf("Create stream: %d\n", streamId)
                        stream := ReceiveStream{}
                        stream.request = func(numElements uint8) {
                                var reqMsg [6]byte
                                reqMsg[0] = MESSAGE_TYPE_STREAM_REQUEST_DATA
                                reqMsg[1] = streamId
                                reqMsg[2] = numElements
                                //binary.BigEndian.PutUint32(reqMsg[2:], numElements)
                                c.send(reqMsg[:])
                        }
                        c.receiveStreams[streamId] = &stream
                        c.streamCallback(&stream)
                case MESSAGE_TYPE_STREAM_DATA:
                        //log.Printf("Data for stream: %d\n", streamId)
                        stream := c.receiveStreams[streamId]
                        stream.dataCallback(message[2:])
                case MESSAGE_TYPE_STREAM_END:
                        log.Printf("End stream: %d\n", streamId)
                        stream := c.receiveStreams[streamId]
                        stream.endCallback()
                case MESSAGE_TYPE_TERMINATE_SEND_STREAM:
                        log.Printf("Terminate stream: %d\n", streamId)
                case MESSAGE_TYPE_STREAM_REQUEST_DATA:
                        log.Printf("Data requested for stream: %d\n", streamId)
                default:
                        log.Printf("Unsupported message type: %d\n", messageType)
                }
        }
}

func (c *Connection) SetSendHandler(callback func([]byte)) {
        c.send = callback
}

func (c *Connection) OnStream(callback func(*ReceiveStream)) {
        c.streamCallback = callback
}


type ReceiveStream struct {
        request func(uint8)
        dataCallback func([]byte)
        endCallback func()
}

func (s *ReceiveStream) OnData(callback func([]byte)) {
        s.dataCallback = callback
}

func (s *ReceiveStream) OnEnd(callback func()) {
        s.endCallback = callback
}

func (s *ReceiveStream) Request(numElements uint8) {
        s.request(numElements)
}



var upgrader = websocket.Upgrader{}


func handler(w http.ResponseWriter, r *http.Request) {

        wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
        defer wsConn.Close()

        writeChan := make(chan []byte)

        conn := CreateConnection()

        writeMessages := func() {
                for message := range writeChan {
                        if err := wsConn.WriteMessage(websocket.BinaryMessage, message); err != nil {
                                log.Println(err)
                                return
                        }
                }
        }
        go writeMessages()

        conn.SetSendHandler(func(message []byte) {
                writeChan <- message
        })

        conn.OnStream(func(stream *ReceiveStream) {

                log.Println("Got a streamy")

                stream.OnData(func(data []byte) {
                        log.Println(len(data))
                        stream.request(1)
                })

                stream.OnEnd(func() {
                        log.Println("end streamy")
                        conn.sendControlMessage([]byte("hi there"))
                })

                stream.request(10)
        })

        for {
                _, p, err := wsConn.ReadMessage()
                if err != nil {
                        log.Println(err)
                        return
                }

                conn.HandleMessage(p)
        }
}

func main() {
        upgrader.CheckOrigin = func(r *http.Request) bool {
                return true
        }

	fmt.Printf("Hi there\n")
        http.HandleFunc("/", handler)
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}
