package main

import (
        "fmt"
        "log"
        "net/http"
        "strings"
        "encoding/json"
        omnicore "github.com/anderspitman/omnistreams-core-go"
        omniconc "github.com/anderspitman/omnistreams-concurrent-go"
        "github.com/satori/go.uuid"
)

type HandshakeMessage struct {
        RequestId byte `json:"requestId"`
        Type string `json:"type"`
        Url string `json:"url"`
}

type StreamMetadata struct {
        Id byte
        Size uint32
}


func main() {
	fmt.Printf("Hi there\n")

        muxes := make(map[uuid.UUID]*omniconc.Multiplexer)

        // TODO: use uint32
        var nextRequestId byte
        nextRequestId = 0

        streamChannels := make(map[byte]chan(omnicore.Producer))

        muxAcceptor := omniconc.CreateWebSocketMuxAcceptor()

        muxAcceptor.OnMux(func(mux *omniconc.Multiplexer) {
                log.Println("got muxy")

                id := uuid.Must(uuid.NewV4())

                fmt.Printf("curl localhost:9001/%s\n", id)

                // TODO: need to delete muxes after connection closes
                muxes[id] = mux

                var idForNextStream byte

                mux.OnControlMessage(func(message []byte) {
                        log.Println("Control message:", message)
                        idForNextStream = message[0]

                        m := make(map[string]string)
                        err := json.Unmarshal(message, m)
                        if err != nil {
                                log.Println("error decoding control message")
                        }
                })

                mux.OnStream(func(stream omnicore.Producer, metadata []byte) {

                        log.Println("Got a streamy, use id", idForNextStream)

                        md := StreamMetadata{}
                        err := json.Unmarshal(metadata, &md)
                        if err != nil {
                                log.Println("error decoding stream metadata")
                        }

                        fmt.Println(metadata)
                        fmt.Println(md)

                        streamChannel := streamChannels[md.Id]

                        if streamChannel != nil {
                                streamChannel <- stream
                        }
                })

                m := make(map[string]string)
                m["type"] = "complete-handshake"
                stringId, err := id.MarshalText()
                if err != nil {
                        log.Println("error decoding uuid")
                }
                m["id"] = string(stringId)

                msg, err := json.Marshal(m)
                if err != nil {
                        log.Println("error encoding handshake")
                }

                mux.SendControlMessage([]byte(msg))
        })

        var httpHandler = func(w http.ResponseWriter, r *http.Request) {
                if r.Method == "GET" {
                        pathParts := strings.Split(r.URL.Path, "/")
                        id := uuid.Must(uuid.FromString(pathParts[1]))
                        // TODO: seems like this probably isn't concurrency-safe
                        mux := muxes[id]

                        if mux != nil {
                                requestId := nextRequestId
                                nextRequestId++

                                streamChannel := make(chan omnicore.Producer)
                                streamChannels[requestId] = streamChannel

                                done := make(chan bool)

                                getReq := HandshakeMessage {
                                        requestId,
                                        "GET",
                                        "/" + strings.Join(pathParts[2:], "/"),
                                }
                                //getReq := make(map[string]string)
                                //getReq["requestId"] = string(requestId)
                                //getReq["type"] = "GET"
                                //getReq["url"] = "/" + strings.Join(pathParts[2:], "/")
                                fmt.Println(getReq)

                                getReqJson, err := json.Marshal(getReq)
                                if err != nil {
                                        log.Println("error encoding GET request")
                                }

                                mux.SendControlMessage([]byte(getReqJson))

                                stream := <-streamChannel

                                stream.OnData(func(data []byte) {
                                        //log.Println(len(data))
                                        w.Write(data)
                                        stream.Request(1)
                                })

                                stream.OnEnd(func() {
                                        log.Println("end streamy")
                                        //mux.SendControlMessage([]byte("all done"))
                                        done <- true
                                })

                                stream.Request(10)

                                <-done
                        } else {
                                w.Write([]byte("invalid uuid"))
                        }

                        log.Println("exit handler")
                } else {
                        w.Write([]byte("Method not supported"))
                }
        }

        http.HandleFunc("/", httpHandler)
        http.HandleFunc("/omnistreams", muxAcceptor.GetHttpHandler())
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}
