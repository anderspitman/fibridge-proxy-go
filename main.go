package main

import (
        "fmt"
        "log"
        "net/http"
        "github.com/anderspitman/omnistreams-concurrent-go"
)


func main() {
	fmt.Printf("Hi there\n")

        muxAcceptor := omniconc.CreateWebSocketMuxAcceptor()

        http.HandleFunc("/", muxAcceptor.GetHttpHandler())
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}
