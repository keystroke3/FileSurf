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
	"strings"
	"syscall"
)

type NetResponse struct {
	Paths string
	Error string
}

func Listen(listener net.Listener) {
	defer listener.Close()
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		if sig != nil {
			listener.Close()
			fmt.Printf("\nclosing open connections...\n")
			os.Exit(1)
		}
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if _, ok := err.(net.Error); ok {
				log.Printf("\nunable to accept connections temporarily: %v\n", err)
				break
			}
			if err != io.EOF {
				log.Printf("\nunable to accept connections, %v\n", err)
				break
			}
		}
		go handleMessage(conn)
	}

}

func TCPListen(addr string) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("unable to listen on address, ", err)
	}
	log.Println("census http daemon listening on", addr)
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
	fmt.Println("another census is already running")
	os.Exit(1)

}

func localizeHomePaths(args *CmdArgs) error {
	clean_paths := []string{}
	for _, path := range args.Paths {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("unable to determine local home directory")
		}
		if strings.HasPrefix(path, "~") {
			clean_paths = append(clean_paths, strings.Replace(path, "~", home, 1))
		} else {
			clean_paths = append(clean_paths, path)
		}
	}
	args.Paths = clean_paths
	return nil

}

func handleMessage(conn net.Conn) {
	defer conn.Close()
	log.Printf("handling message from %v\n", conn.RemoteAddr())
	var cmd CmdArgs
	buf := make([]byte, 1024)
	_, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		log.Printf("unable to read from socket connection, %v\n", err)
		return
	}
	resp := NetResponse{}
	rcmd := bytes.Trim(buf, "\x00")
	err = json.Unmarshal(rcmd, &cmd)
	if err != nil {
		log.Println("unable to read command", err)
		resp.Error = fmt.Sprintf("unable to read command. %v", err)
		respb, merr := json.Marshal(resp)
		if merr != nil {
			log.Println("unable to marshal response, ", merr)
			return
		}
		conn.Write(respb)
		return
	}

	log.Printf("command args: %+v", cmd)

	err = localizeHomePaths(&cmd)
	if err != nil {
		resp.Error = err.Error()
		respb, merr := json.Marshal(resp)
		if merr != nil {
			log.Println("unable to marshal response, ", merr)
			return
		}
		conn.Write(respb)
		return
	}
	result, err := handleCommand(cmd)
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Paths = result
	}

	// if strings.HasPrefix(result, "path not found") {
	// 	result = strings.Replace(result,
	// 		"path not found",
	// 		fmt.Sprintf("path not found on %v ", conn.LocalAddr()),
	// 		1,
	// 	)
	// }
	respb, err := json.Marshal(resp)
	if err != nil {
		log.Println("unable to marshal response, ", err)
		return
	}
	_, err = conn.Write(respb)
	if err != nil {
		log.Println("unable to write response to client")
	}
}
