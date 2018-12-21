package main

import (
        "fmt"
        "log"
        "net/http"
        "github.com/anderspitman/omnistreams-core-go"
        "github.com/anderspitman/omnistreams-concurrent-go"
)


func main() {
	fmt.Printf("Hi there\n")

        muxAcceptor := omniconc.CreateWebSocketMuxAcceptor()

        muxAcceptor.OnMux(func(mux *omniconc.Multiplexer) {
                log.Println("got muxy")
                mux.OnStream(func(stream omnicore.Producer) {

                        log.Println("Got a streamy")

                        stream.OnData(func(data []byte) {
                                log.Println(len(data))
                                stream.Request(1)
                        })

                        stream.OnEnd(func() {
                                log.Println("end streamy")
                                mux.SendControlMessage([]byte("hi there"))
                        })

                        stream.Request(10)
                })
        })

        http.HandleFunc("/", muxAcceptor.GetHttpHandler())
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}
