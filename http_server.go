package main

/*
-- run by standing in http_server/ and:
go run . 8080
(or: go build -o http_server . && ./http_server 8080)

-- check connection by running this in another terminal:
nc localhost 8080
for i in $(seq 1 20); do nc localhost 8080 & done
curl -v http://localhost:8080/
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
	//"time" // for testing go routines
)

func main() {
	port, err := getPort()
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // main is the right place to exit
	}

	// listen to a socket
	ln, err := net.Listen("tcp", ":"+port)
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
		go handleConnection(conn)
	}
}

// --------------------------------------------------------------------

func handleConnection(conn net.Conn) {
	defer conn.Close()
	// -- for server --
	fmt.Println("established connection with:", conn.RemoteAddr().String())
	// -- translate client's  http msg into a http struct
	reader := bufio.NewReader(conn) // takes the incoming bytes and creates a reader
	request, err := http.ReadRequest(reader) // formats the http msg
	if err != nil {
		fmt.Println("400!") // TO DO actual 400
		return
	}
	// deal with the incoming http struct
	method := request.Method // GET or POST
	var response *http.Response
    switch method {
		case "GET":
	    	response = getResponse(request)
		case "POST":
			response = postResponse(request)
		default:
			response = wellFormed("501 Not Implemented", 501, "text/plain", "not implemented\n")
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
        ContentLength: int64(len(body)),
        Close:         true,
    }
}

func getResponse(request *http.Request) *http.Response {
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, ".")
    var body string
    if !found {
        return wellFormed("404 Not Found", 404, filetype, "hello")
    }
    switch filetype {
        case "html", "txt", "gif", "jpeg", "jpg", "css":
            body, err := findFile(path)
       		if err != nil {
				return wellFormed("404 Not Found", 404, filetype, body)
			}
			return wellFormed("200 OK", 200, filetype, "hello")
        default:
            return wellFormed("400 Bad Request", 400, filetype, body)
    }
}

// TO DO just a copy of getResponse for now
func postResponse(request *http.Request) *http.Response {
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, ".")
    var body string
    if !found {
        return wellFormed("404 Not Found", 404, filetype, "hello")
    }
    switch filetype {
        case "html", "txt", "gif", "jpeg", "jpg", "css":
            body, err := os.ReadFile(path) 
       		if err != nil {
				return wellFormed("404 Not Found", 404, filetype, "hello")
			}
			return wellFormed("200 OK", 200, filetype, body)
        default:
            return wellFormed("400 Bad Request", 400, filetype, body)
    }
}

/*
func findFile(path string) (file_content []byte, err error) {
    file, err := os.Open(path)
    if err != nil {
        return nil, errors.New("file not found")
    }

    var data []byte
    buf := make([]byte, 1024)
    
    n, err := file.Read(buf)
    return file.Read, nil 
}
*/

func findFile(path string) (content string, err error) {
	file, err := os.Open(path)



	content, err := os.Readfile(file)


	
	if err != nil {
	    return "", errors.New("file not found")
	} 
	n, err := file.Read(content)
	if err != nil & n != 0 {
	    return "", errors.New("file empty")
	} 	
	return string(content), nil
}

// --------------------------------------------------------------------

