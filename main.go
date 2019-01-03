package main

import (
        "fmt"
        "log"
        "net/http"
        "strings"
        "strconv"
        "encoding/json"
        omnicore "github.com/anderspitman/omnistreams-core-go"
        omniconc "github.com/anderspitman/omnistreams-concurrent-go"
        omniwriter "github.com/anderspitman/omnistreams-writer-adapter-go"
        "github.com/satori/go.uuid"
)

type GetRequestMessage struct {
        RequestId byte `json:"requestId"`
        Type string `json:"type"`
        Url string `json:"url"`
        Range *HttpRange `json:"range,omitempty"`
}

type HttpRange struct {
        Start uint64 `json:"start"`
        // Note: if end is 0 it won't be included in the json because of omitempty
        End uint64 `json:"end,omitempty"`
}

type StreamMetadata struct {
        Id byte `json:"id"`
        Size uint64 `json:"size"`
}

type ControlMessage struct {
        Type string `json:"type"`
        Code int `json:"code"`
        RequestId byte `json:"requestId"`
        Message string `json:"message"`
}

type RequestResult struct {
        success bool
        producer omnicore.Producer
        metadata *StreamMetadata
}

// This is measured in omnistreams elements. In the current fibridge
// implementation each element is a 1MiB chunk, so this buffer translates
// to 100MiB
const BUFFER_SIZE uint32 = 100


func main() {
        fmt.Println("Started")

        muxes := make(map[uuid.UUID]*omniconc.Multiplexer)

        // TODO: use uint32
        var nextRequestId byte
        nextRequestId = 0

        streamChannels := make(map[byte]chan(RequestResult))

        muxAcceptor := omniconc.CreateWebSocketMuxAcceptor()

        muxAcceptor.OnMux(func(mux *omniconc.Multiplexer) {

                id := uuid.Must(uuid.NewV4())

                // TODO: need to delete muxes after connection closes
                muxes[id] = mux

                mux.OnControlMessage(func(message []byte) {
                        m := ControlMessage{}
                        err := json.Unmarshal(message, &m)
                        if err != nil {
                                log.Println(err)
                        }

                        log.Println("Control message:")
                        log.Println(m)

                        if m.Type == "error" {
                                streamChannel := streamChannels[m.RequestId]

                                if streamChannel != nil {
                                        streamChannel <- RequestResult {
                                                false,
                                                nil,
                                                nil,
                                        }
                                }
                        }
                })

                mux.OnConduit(func(producer omnicore.Producer, metadata []byte) {

                        md := StreamMetadata{}
                        err := json.Unmarshal(metadata, &md)
                        if err != nil {
                                log.Println(err)
                        }

                        streamChannel := streamChannels[md.Id]

                        if streamChannel != nil {
                                streamChannel <- RequestResult {
                                        true,
                                        producer,
                                        &md,
                                }
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

                                streamChannel := make(chan RequestResult)
                                streamChannels[requestId] = streamChannel

                                done := make(chan bool)

                                httpRange := parseRange(r.Header["Range"])

                                getReq := GetRequestMessage {
                                        requestId,
                                        "GET",
                                        "/" + strings.Join(pathParts[2:], "/"),
                                        httpRange,
                                }
                                //getReq := make(map[string]string)
                                //getReq["requestId"] = string(requestId)
                                //getReq["type"] = "GET"
                                //getReq["url"] = "/" + strings.Join(pathParts[2:], "/")

                                getReqJson, err := json.Marshal(getReq)
                                if err != nil {
                                        log.Println("error encoding GET request")
                                }

                                fmt.Println(string(getReqJson))

                                mux.SendControlMessage([]byte(getReqJson))

                                // TODO: finished might not be needed
                                finished := false

                                ended := false

                                go func() {
                                        <-r.Context().Done()

                                        finished = true

                                        if ended {
                                                log.Println("Ended normally")
                                                done <- true
                                        } else {
                                                log.Println("Disconnected")
                                                done <- false
                                        }
                                }()

                                // Will block here until client receives request and starts streaming
                                result := <-streamChannel

                                if result.success {
                                        if httpRange != nil {
                                                w.Header()["Content-Range"] = buildRangeHeader(httpRange, result.metadata.Size)
                                        }

                                        producer := result.producer
                                        consumer := omniwriter.CreateWriterAdapter(w, uint32(BUFFER_SIZE))
                                        consumer.OnFinished(func() {
                                                ended = true
                                                finished = true
                                                done <- true
                                        })

                                        producer.Pipe(consumer)
                                        producer.Request(BUFFER_SIZE)

                                        //dataChan := make(chan []byte, BUFFER_SIZE)

                                        //producer.OnData(func(data []byte) {
					//	dataChan <- data
                                        //})

				        //go func() {
					//        for data := range dataChan {
					//	        if !finished {
					//			w.Write(data)
					//			producer.Request(1)
					//		}
					//	}

                                        //        ended = true
                                        //        finished = true
                                        //        done <- true
					//}()

                                        //producer.OnEnd(func() {
                                        //        close(dataChan)
                                        //})

                                        //producer.OnCancel(func() {
                                        //        fmt.Println("Yo man I been canceled")
                                        //})

                                        //producer.Request(BUFFER_SIZE)


                                        endedNormally := <-done

                                        if !endedNormally {
                                                producer.Cancel()
                                        }
                                } else {
                                        w.WriteHeader(http.StatusNotFound)
                                }
                        } else {
                                w.Write([]byte("invalid uuid"))
                        }

                } else {
                        w.Write([]byte("Method not supported"))
                }
        }

        http.HandleFunc("/", httpHandler)
        http.HandleFunc("/omnistreams", muxAcceptor.GetHttpHandler())
        log.Fatal(http.ListenAndServe("localhost:9001", nil))
}

func parseRange(header []string) *HttpRange {

        if len(header) > 0 {

                // TODO: this is very hacky and brittle
                parts := strings.Split(header[0], "=")
                rangeParts := strings.Split(parts[1], "-")

                start, err := strconv.Atoi(rangeParts[0])
                if err != nil {
                        log.Println("Decode range start failed")
                }
                end, err := strconv.Atoi(rangeParts[1])
                if err != nil {
                        log.Println("Decode range end failed")
                }
                return &HttpRange {
                        Start: uint64(start),
                        End: uint64(end),
                }
        } else {
                return nil
        }
}

func buildRangeHeader(r *HttpRange, size uint64) []string {

        if r.End == 0 {
                r.End = size - 1
        }

        contentRange := fmt.Sprintf("bytes %d-%d/%d", r.Start, r.End, size)
        return []string{contentRange}
}
