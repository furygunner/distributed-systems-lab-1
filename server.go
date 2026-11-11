package main

import (
    "net"
    "os"
    "fmt"
    "net/http"
    "bufio"
    "io"
    "net/textproto"
    "strings"
)

func main() {
    ln, err := net.Listen("tcp", os.Args[1])
    if err != nil {
        panic("whoopsie")
    }
    for {
        conn, err := ln.Accept()
        if err != nil {
            panic("")
        }
        go handleConnection(conn)
    }
}

func handleConnection(conn net.Conn) {
    var b = make([]byte, 1000)
    reader := bufio.NewReader(conn)
    tp := textproto.NewReader(reader)
    request, _ := http.ReadRequest(reader)
    method := request.Method

    switch method {
        case "GET":
            response := getResponse(request)
        case "POST":
            response := postResponse(request)
        default:
    }
}

func getResponse(request *http.Request) http.Response {
    path := request.URL.EscapedPath()
    _, filetype, found := strings.CutLast(path, '.')
    if !found {
        return wellFormed(404, "404 Not Found")
    }
    switch filetype {
        case "html", "txt", "gif", "jpeg", "jpg", "css":
             := findFile(path)
        default:
            return wellFormed(400, "400 Bad Request")
    }
}

func wellFormed(code int, status string) http.Response {
    return http.Response{
        status,
        code,
        "HTTP/1.0", 1, 0,
        http.Header{},
        io.NopCloser(strings.NewReader("hello")),
        5,
        nil,
        true,
        true,
        http.Header{},
        nil,
        nil,
    }
}
