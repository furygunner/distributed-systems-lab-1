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
	//"time" // for testing go routines
)

// --------------------------------------------------------------------

func handleConnection(conn net.Conn) {
	defer conn.Close()
	// -- for local --
	fmt.Println("established connection with:", conn.RemoteAddr().String())

	// -- for remote --
	body := "hello client! You are connected to: " + conn.LocalAddr().String() + "\n"
	resp := "HTTP/1.1 200 OK\r\n" +
		"Content-Type: text/plain\r\n" +
		"Content-Length: " + strconv.Itoa(len(body)) + "\r\n" +
		"Connection: close\r\n" +
		"\r\n" + // blank line = end of headers
		body
	conn.Write([]byte(resp))
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

// --------------------------------------------------------------------

func main() {
	port, err := getPort()
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // main is the right place to exit
	}

	// listen to a socket
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println(err)
		os.Exit(1) // can't continue without a listener
	}
	defer ln.Close()
	fmt.Println("listening on port", port)

	// Accept each connection
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println(err)
			continue
		}
		go handleConnection(conn)
	}
}
