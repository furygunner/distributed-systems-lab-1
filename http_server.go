package main

/*
-- run by standing in http_server/ and:
go run . 8080
(or: go build -o http_server . && ./http_server 8080)

-- check connection by running this in another terminal:
nc localhost 8080
for i in $(seq 1 20); do nc localhost 8080 & done
curl -v http://localhost:8080/


curl -i -X POST "localhost:8080/subscribe?topic=news"
*/

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"bufio"
	"net/http"
    "io"
    "strings"
    "bytes"
    "sync"
	"time" 
)


func main() {

	broker := newBrokerContext()
	broker.channel = make(chan struct{}, 10) // We want to use 10 channels

	port, err := getPort()
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // main is the right place to exit
	}

	// listen to a socket
	ln, err := net.Listen("tcp", ":" + port)
	if err != nil { // ASK what does panic mean as an error?
		fmt.Println(err)
		os.Exit(1) // can't continue without a listener
	}
	defer ln.Close() // ASK why does ada not have defer?
	fmt.Println("listening on port", port)

	// Accept each connection
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue // ASK why does ada not have continue?
		}
		// send a struct containing nothing to occupy one of the 10 slots
		// as a marker
		broker.channel <- struct{}{} // Occupy one of the 10 slots
		// define a varaiable that when called releases one of the markers
		// OnceFunc makes sure that release is only called once
		release := sync.OnceFunc(func() { <-broker.channel })
		go func() { // Run concurrently
			defer release() // release will run here if no other release calls happens
			fmt.Println("Handler started")
			handleConnection(conn, broker, release)
		}()
	}
}

// __________________ PUBSUB HEADER_____________________________

	type Message struct {
		seq  int // order of each msg chunk
		body string // on chunk
	}

	type Subscriber struct {
		id      	int
		topic   	string 
		msg_channel chan Message // buffered queue that goroutines take msgs 
		}						// from when they are available, otherwise sleep

	type BrokerContext struct {
		mu          sync.Mutex // only touch the subscriber list when you have the mutex 
		nextID      int // for when creating new subscribers
		nextSeq     map[string]int64      // topic -> last sequence number in Message
		subscribers map[int]*Subscriber   // id -> subscriber; loop through these whenever a publish occures
		channel 	chan struct{}
	}

	func newBrokerContext() *BrokerContext {
		return &BrokerContext{ subscribers: make(map[int]*Subscriber), nextSeq: make(map[string]int64), }
	}

// ------------------------------ FUNCTIONS --------------------------------------

func handleConnection(conn net.Conn, broker *BrokerContext, release func()) {
	defer conn.Close()
	// -- for server --
	fmt.Println("established connection with:", conn.RemoteAddr().String())
	// -- translate client's  http msg into a http struct
	reader := bufio.NewReader(conn) // takes the incoming bytes and creates a reader
	// abort if this connection is taking too long
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var response *http.Response
	request, err := http.ReadRequest(reader) // formats the http msg
	if err != nil {
		response = wellFormed("400 Bad Request", 400, "text/plain", "request couldnt be read\n")	
	} else {
		// deal with the incoming http struct
       	response = createResponse(request, broker, release)
	}
	// sent the reponse to client
    response.Write(conn)
}

// getPort reads the port from the first command line argument
// and returns an error if it is missing or invalid.
func getPort() (string, error) {
	if len(os.Args) != 2 { // there should only be the prg name and 1 arg as input
		return "", errors.New("usage: ./http_server <port>")
	}
	port := os.Args[1] // the arg is the port
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 { // only return acceptable
		return "", errors.New("port must be a number between 1 and 65535")
	}
	return port, nil // nil means no error
}

func wellFormed(status string, status_code int, contentType string, body string) *http.Response {
    return &http.Response{
        Status:        status, 
        StatusCode:    status_code,
        Proto:         "HTTP/1.1",
        ProtoMajor:    1,
        ProtoMinor:    1,
        Header:        http.Header{"Content-Type": {contentType}},
        Body:          io.NopCloser(bytes.NewReader([]byte(body))),
        ContentLength: -1,
        Close:         true,
    }
}

func createResponse(request *http.Request, broker *BrokerContext, release func()) *http.Response {
	// does this request contain a file?
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, ".")
	method := request.Method
    if method != "GET" && method != "POST" {
    	wellFormed("501 Not Implemented", 501, "text/plain", "only GET or POST is implemented\n")
    }
    if !found {
    	// requests without a file is handled by the broker
    	return handlePubSubAction(request, broker, release)
	}
	// check if the file.* is acceptable
	contentType, wellFormedError := correctContentType(filetype)
	if wellFormedError != nil {
		return wellFormedError
	}
	// check that a supported method is used and handle accordingly
	switch method {
		case "GET":
			body, err := findFile(path)
			if err != nil {
				return wellFormed("404 Not Found", 404, contentType, "couldnt find file\n")
			}
			return wellFormed("200 OK", 200, contentType, body)
		case "POST":
			data, err := io.ReadAll(request.Body)
			if err != nil {
				return wellFormed("400 Bad Request", 400, "text/plain", "bad body\n")
			}
			err = postFile(path, data)
			if err != nil {
				return wellFormed("500 Internal Server Error", 500, "text/plain", "write Failed\n")
			}
			return wellFormed("201 Created", 201, "text/plain", "successful post")
		default:
			return wellFormed("501 Not Implemented", 501, "text/plain", "only GET or POST is implemented\n")
	}
}

func correctContentType(filetype string) (contentType string, wellFormedError *http.Response) {    
    switch filetype {
		case "html":
			contentType = "text/html"
		case "txt":
			contentType = "text/plain"
		case "gif":
			contentType = "image/gif"
		case "jpeg":
			contentType = "image/jpeg"
		case "jpg":
			contentType = "image/jpeg"
		case "css":
			contentType = "text/css"
        default:
	        wellFormedError = wellFormed("400 Bad Request", 400, "text/plain", "incorrent contentType\n")
	}
	return contentType, wellFormedError
}

func handlePubSubAction(request *http.Request, broker *BrokerContext, release func())  *http.Response {
	topic := request.URL.Query().Get("topic")
	if topic == "" {
		return wellFormed("400 Bad Request", 400, "text/plain", "no topic\n")
	}
	switch request.URL.Path {      
		case "/subscribe":
			if request.Method != "POST" {
				return wellFormed("501 Not Implemented", 501, "text/plain", "subscribe must be post\n")
			}
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// create a subscriber
			broker.nextID++
			id := broker.nextID
			broker.subscribers[id] = &Subscriber{id: id, topic: topic, msg_channel: make(chan Message, 16)}
			return wellFormed("201 Created", 201, "text/plain", fmt.Sprintf("{\"id\": %d}\n", id))
			
		case "/unsubscribe":
			if request.Method != "POST" {
				return wellFormed("501 Not Implemented", 501, "text/plain", "unsubscribe must be post\n")
			}		
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// delete the subscriber 
			id, err := strconv.Atoi(request.URL.Query().Get("id"))
			if err != nil {
				return wellFormed("400 Bad Request", 400, "text/plain", "in correct formatted id\n")
			}
			_, ok := broker.subscribers[id]
			if !ok {
				return wellFormed("404 Not Found", 404, "text/plain", "cannot delete unknown subscriber\n")
			}
			delete(broker.subscribers, int(id))
			return wellFormed("200 OK", 200, "text/plain", fmt.Sprintf("{\"unsubscribed\": %d}\n", id)) 

		case "/publish":
			if request.Method != "POST" {
				return wellFormed("501 Not Implemented", 501, "text/plain", "publish must be post\n")
			}
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// create a msg
			body, _ := io.ReadAll(request.Body)
			broker.nextSeq[topic]++
			seq := broker.nextSeq[topic]
			msg := Message{seq: int(seq), body: string(body)}

			for _, sub := range broker.subscribers {
				if sub.topic == topic {
					select { // let this goroutine do 1 of these things depending on which is ready first
						case sub.msg_channel <- msg: // put the msg in the buffer
						default: // buffer is full -> skip this sub 
					}
				}
			}
			return wellFormed("200 OK", 200, "text/plain", fmt.Sprintf("{\"published\": %s}\n", topic))
			
		case "/events":
			if request.Method != "GET" {
				return wellFormed("501 Not Implemented", 501, "text/plain", "events must be get\n")
			}
			// try to get id
			id, err := strconv.Atoi(request.URL.Query().Get("id"))
			if err != nil {
				return wellFormed("400 Bad Request", 400, "text/plain", "id couldnt be used\n")
			}
			// get the state of this subscriber safely
			broker.mu.Lock() 
			sub, ok := broker.subscribers[id]
			broker.mu.Unlock()
			// if the subscriber doesnt exists or the topic doesnt match
			var msg_output string
			if !ok || sub.topic != topic {
				return wellFormed("404 Not Found", 404, "text/plain", "unknown subscriber\n")
			}
			// before waiting, release the connection slot so us waiting doesnt count as one of the
			// 10 routines that are allowed to run concurrently
			release()
			select { // WAIT until one of these things are possible (no default)
				case m := <-sub.msg_channel: // a msgs exists in in the subscribers msg buffer
					msg_output += fmt.Sprintf("%d %s\n", m.seq, m.body)
				case <-time.After(30*time.Second): // 30 sec elapsed -> timeout
					return wellFormed("204 No Content", 204, "text/plain", "took too long\n")
			}
			for {
				select {
				case m := <-sub.msg_channel: // a msgs exists in in the subscribers msg buffer
					msg_output += fmt.Sprintf("%d %s\n", m.seq, m.body)
				default: // finish when no msg remain
					return wellFormed("200 OK", 200, "text/plain", msg_output + "\n")
				}				
			}
		default:
			return wellFormed("400 Bad Request", 400, "text/plain", "not a valid pubsub actio\n")
		}
}

func findFile(path string) (string, error) {
    relative, _ := strings.CutPrefix(path, "/")
    fmt.Println(relative) 
    // abort if the path contains anything that changes the directory
    if strings.Contains(relative, "//") || strings.Contains(relative, "..") {
    	return "", errors.New("faulty path")
   	}
    data, err := os.ReadFile(relative)
    if err != nil {
        return "", errors.New("file not found")
    }
    return string(data), nil
}

func postFile(path string, data []byte) error {
	// find path/filename.* relative to this project 
    relative, _ := strings.CutPrefix(path, "/")
    // abort if the path contains anything that changes the directory
    if strings.Contains(relative, "//") || strings.Contains(relative, "..") {
    	return errors.New("faulty path")
   	}
	// try to create a file in an atomic way in the project dir
	// we name is something temporary that no program can reference
	// * gets replaced by random digits
	// "." created the file at root
    temp, err := os.CreateTemp(".", ".upload-*")
    if err != nil {
        return err
    }
    // if this func does not succeed -> remove temp
    defer os.Remove(temp.Name()) 

	// try to fill temp with data
	_, err = temp.Write(data)
    if err != nil {
        temp.Close()
        return err
    }
    // try to close temp
    err = temp.Close()
    if err != nil {
        return err
    }
    // Rename temp to a real name atomically
    return os.Rename(temp.Name(), relative) 
}

// --------------------------------------------------------------------

