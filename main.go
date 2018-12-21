package main

import (
        "fmt"
        "log"
        "net/http"
        omnicore "github.com/anderspitman/omnistreams-core-go"
        omniconc "github.com/anderspitman/omnistreams-concurrent-go"
        "github.com/satori/go.uuid"
)


func main() {
	fmt.Printf("Hi there\n")

        muxes := make(map[uuid.UUID]*omniconc.Multiplexer)

        muxAcceptor := omniconc.CreateWebSocketMuxAcceptor()

        muxAcceptor.OnMux(func(mux *omniconc.Multiplexer) {
                log.Println("got muxy")

                id := uuid.Must(uuid.NewV4())

                log.Println(id)

                // TODO: need to delete muxes after connection closes
                muxes[id] = mux

                mux.OnControlMessage(func(message []byte) {
                        log.Println("Control message:", message)
                })

                mux.OnStream(func(stream omnicore.Producer) {

                        log.Println("Got a streamy")

                        stream.OnData(func(data []byte) {
                                log.Println(len(data))
                                stream.Request(1)
                        })

                        stream.OnEnd(func() {
                                log.Println("end streamy")
                                mux.SendControlMessage([]byte("all done"))
                        })

                        stream.Request(10)
                })
        })

        http.HandleFunc("/", muxAcceptor.GetHttpHandler())
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}
