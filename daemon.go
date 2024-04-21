package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)


type NetResponse struct {
	Paths string
	Error string
}

func Listen(listener net.Listener){
    defer listener.Close()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
        listener.Close()
		os.Exit(1)
	}()

	for {
		conn, err := listener.Accept()
		if err != nil && err != io.EOF {
			log.Printf("unable to read from connection, %s", err)
			continue
		}
		go handleMessage(conn)
	}
}

func TCPListen(addr string){
    listener, err := net.Listen("tcp", addr)
    if err != nil{
        log.Fatal("unable to listen on address, ", err)
    }
    log.Println("filesurf http daemon listening on", addr)
    Listen(listener)
}

func UnixListen(file string) {
	checkSocketConflict(file)
	listener, err := net.Listen("unix", file)
    defer os.Remove(file)
	if err != nil {
		log.Fatal("unable to initialize the daemon ", err)
	}
    Listen(listener)
}

func checkSocketConflict(file string) {
	_, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Fatal("unable to check for daemon conflicts, ", err)
	}
	conn, err := net.Dial("unix", file)
	defer conn.Close()
	if err != nil {
		os.Remove(file)
		return
	}
	fmt.Println("another filesurf is already running")
	os.Exit(1)

}

func handleMessage(conn io.ReadWriteCloser) {
	defer conn.Close()
	var cmd CmdArgs
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		fmt.Printf("unable to read from socket connection, %v\n", err)
		return
	}
	rcmd := bytes.Trim(buf, "\x00")
	err = json.Unmarshal(rcmd, &cmd)
	if err != nil {
		resp := NetResponse{
			Paths: "",
            Error: fmt.Sprintf(`{"Error": "unable to read command. %v"}`, err),
		}
		respb, merr := json.Marshal(resp)
		if merr != nil {
			log.Println("unable to marshal response, ", merr)
			return
		}
		conn.Write(respb)
        return
	}
	result := handleCommand(cmd)
    resp := NetResponse{
		Paths: result,
		Error: "",
	}
    respb, err := json.Marshal(resp)
    _, err = conn.Write(respb)
    if err != nil{
        log.Println("unable to write response to client")
    }
}

