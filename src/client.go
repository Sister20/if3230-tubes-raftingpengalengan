package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"rafting/lib"
	"strings"
)

type Client struct {
	clients []*rpc.Client
	addrs   []string
}

func NewClient(addrs []string) *Client {
	clients := make([]*rpc.Client, len(addrs))
	for i, addr := range addrs {
		conn, err := net.DialTimeout("tcp", addr, lib.RpcTimeout)
		if err != nil {
			log.Fatalf("Dialing failed: %v", err)
		}
		clients[i] = rpc.NewClient(conn)
	}
	return &Client{clients: clients, addrs: addrs}
}

func (c *Client) Call(i int, serviceMethod string, args interface{}, reply interface{}) error {
	return c.clients[i].Call(serviceMethod, args, reply)
}

func (c *Client) CallAll(serviceMethod string, args interface{}, reply interface{}) []error {
	errs := make([]error, len(c.clients))
	for i, client := range c.clients {
		errs[i] = client.Call(serviceMethod, args, reply)
	}
	return errs
}

func main() {
	//addrs := []string{
	//	"localhost:8080",
	//	"localhost:8081",
	//	"localhost:8082",
	//	"localhost:8083",
	//	"localhost:8084",
	//	"localhost:8085",
	//}
	//client := NewClient(addrs)
	//
	reader := bufio.NewReader(os.Stdin)

	// Use client.Call and client.CallAll to send requests
	for {
		line, _ := reader.ReadString('\n')
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]
		switch cmd {
		case "append":
			if len(parts) < 3 {
				fmt.Println("Not enough arguments for append")
				continue
			}
			key := parts[1]
			value := parts[2]
			fmt.Println("Appending to key", key, "value", value)
		case "set":
			if len(parts) < 3 {
				fmt.Println("Not enough arguments for set")
				continue
			}
			key := parts[1]
			value := parts[2]
			fmt.Println("Setting key", key, "to value", value)
		case "strlen":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for strlen")
				continue
			}
			key := parts[1]
			fmt.Println("Getting length of key", key)
		case "del":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for del")
				continue
			}
			key := parts[1]
			fmt.Println("Deleting key", key)
		default:
			fmt.Println("Unknown command:", cmd)
		}
	}
}
