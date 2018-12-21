package main

import (
        //"time"
        "fmt"
        "log"
        "net/http"
        //"encoding/binary"
        "github.com/anderspitman/omnistreams-core-go"
        "github.com/anderspitman/omnistreams-concurrent-go"
        "github.com/gorilla/websocket"
)


var upgrader = websocket.Upgrader{}


func handler(w http.ResponseWriter, r *http.Request) {

        wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
        defer wsConn.Close()

        writeChan := make(chan []byte)

        conn := omniconc.CreateConnection()

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

        conn.OnStream(func(stream omnicore.Producer) {

                log.Println("Got a streamy")

                stream.OnData(func(data []byte) {
                        log.Println(len(data))
                        stream.Request(1)
                })

                stream.OnEnd(func() {
                        log.Println("end streamy")
                        conn.SendControlMessage([]byte("hi there"))
                })

                stream.Request(10)
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
