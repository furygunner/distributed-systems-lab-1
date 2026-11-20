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
	//"time" // for testing go routines
)


func main() {

	broker := newBrokerContext()
	
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

	channel := make(chan struct{}, 10) // We want to use 10 channels

	// Accept each connection
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue // ASK why does ada not have continue?
		}
		channel <- struct{}{} // Occupy one of the 10 slots
		go func() { // Run concurrently
			defer func() { <-channel }() // Release the slot when finished
			fmt.Println("Handler started")
			handleConnection(conn, broker)
		}()
	}
}

// --------------------------------------------------------------------

func handleConnection(conn net.Conn, broker *BrokerContext) {
	defer conn.Close()
	// -- for server --
	fmt.Println("established connection with:", conn.RemoteAddr().String())
	// -- translate client's  http msg into a http struct
	reader := bufio.NewReader(conn) // takes the incoming bytes and creates a reader
	request, err := http.ReadRequest(reader) // formats the http msg
	var response *http.Response
	if err != nil {
		response = wellFormed("400 Bad Request", 400, "text/plain", "in handlePubSubAction/topic\n")
	}
	// deal with the incoming http struct
	method := request.Method // GET or POST
    switch method {
		case "GET":
        	response = getResponse(request, broker)
		case "POST":
			response = postResponse(request, broker)
		default:
			response = wellFormed("501 Not Implemented", 501, "text/plain", "in handleConnection\n")
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
        Status:        status, // e.g. "404 Not Found"
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

func getResponse(request *http.Request, broker *BrokerContext) *http.Response {
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, ".")
    if !found {
    	// check to see if its a subscribe or post
    	_, command, found := strings.CutLast(path, "/")
    	if !found {
	        return wellFormed("404 Not Found", 404, filetype, "in getResonse\n")	
    	}
    	return handlePubSubAction(request, command, broker)
    }
    switch filetype {
        case "html", "txt", "gif", "jpeg", "jpg", "css":
            body, err := findFile(path)
       		if err != nil {
				return wellFormed("404 Not Found", 404, filetype, "in getResonse/filetype\n")
			}
			return wellFormed("200 OK", 200, filetype, body)
        default:
            return wellFormed("400 Bad Request", 400, filetype, "in getResonse/filetype\n")
    }
}

// TO DO just a copy of getResponse for now
func postResponse(request *http.Request, broker *BrokerContext) *http.Response {
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, ".")
    if !found {
    	// check to see if its a subscribe or post
    	_, command, found := strings.CutLast(path, "/")
    	if !found {
	        return wellFormed("404 Not Found", 404, filetype, "in postResponse\n")	
    	}
    	return handlePubSubAction(request, command, broker)
	}
    switch filetype {
        case "html", "txt", "gif", "jpeg", "jpg", "css":
			data, err := io.ReadAll(request.Body)
            if err != nil {
            	return wellFormed("400 Bad Request", 400, "text/plain", "bad body\n")
			}
			err = postFile(path, data)
			if err != nil {
				return wellFormed("500 Internal Server Error", 500, "text/plain", "write Failed\n")
			}
			return wellFormed("201 Created", 201, "text/plain", "success")
        default:
            return wellFormed("400 Bad Request", 400, "text/plain", "in postResponse/filetype\n")
    }
}



// __________________ PUBSUB _____________________________

	type Message struct {
		seq  int // order of each msg chunk
		body string // on chunk
	}

	type Subscriber struct {
		id      int
		topic   string 
		pending_msgs []Message // everything waiting to be sent to this subscriber
	}

	type BrokerContext struct {
		mu          sync.Mutex // only touch the subscriber list when you have the mutex 
		nextID      int // for when creating new subscribers
		nextSeq     map[string]int64      // topic -> last sequence number in Message
		subscribers map[int]*Subscriber   // id -> subscriber; loop through these whenever a publish occures
	}

	func newBrokerContext() *BrokerContext {
		return &BrokerContext{ subscribers: make(map[int]*Subscriber), nextSeq: make(map[string]int64), }
	}

func handlePubSubAction(request *http.Request, command string, broker *BrokerContext)  *http.Response {
	topic := request.URL.Query().Get("topic")
	if topic == "" {
		return wellFormed("400 Bad Request", 400, "text/plain", "in handlePubSubAction/topic\n")
	}
	switch request.URL.Path {      
		case "/subscribe":
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// create a subscriber
			broker.nextID++
			id := broker.nextID
			broker.subscribers[id] = &Subscriber{id: id, topic: topic}
			return wellFormed("201 Created", 201, "text/plain", fmt.Sprintf("{\"id\": %d}\n", id))
			
		case "/unsubscribe":
			// DUMMY
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// delete the subscriber 
			id, err := strconv.ParseInt(request.URL.Query().Get("id"), 10, 32)
			if err != nil {
				return wellFormed("400 Bad Request", 400, "text/plain", "in handlePubSubAction/unsubscribe\n")
			}
			delete(broker.subscribers, int(id))
			return wellFormed("200 OK", 200, "text/plain", fmt.Sprintf("{\"unsubscribed\": %d}\n", id)) 

		case "/publish":
			// DUMMY
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
					sub.pending_msgs = append(sub.pending_msgs, msg)
					fmt.Println("queued for subscriber", sub.id, "pending:", len(sub.pending_msgs))
				}
			}
			return wellFormed("200 OK", 200, "text/plain", fmt.Sprintf("{\"published\": %s}\n", topic))
			
		case "/events":
			// DUMMY
			// get the lock in order to perform action
			broker.mu.Lock()
			defer broker.mu.Unlock()
			// create a subscriber
			broker.nextID++
			id := broker.nextID
			broker.subscribers[id] = &Subscriber{id: id, topic: topic}
			return wellFormed("201 Created", 201, "text/plain", fmt.Sprintf("{\"id\": %d}\n", id))
			
		default:
			return wellFormed("400 Bad Request", 400, "text/plain", "in handlePubSubAction/default\n")
		}
}
	 

// __________________ PUBSUB _____________________________


func findFile(path string) (string, error) {
    relative, _ := strings.CutPrefix(path, "/")
    fmt.Println(relative) 
    data, err := os.ReadFile(relative)
    if err != nil {
        return "", errors.New("file not found")
    }
    return string(data), nil
}

func postFile(path string, data []byte) error {
	// find path/filename.* relative to this project 
    relative, _ := strings.CutPrefix(path, "/")
	// create a file in an atomic way in the project dir
	// we name is something temporary that no program can reference
	// * gets replaced by random digits
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

