package main

import (
	"bufio"
	"encoding/json"
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

func (c *Client) Call(i int, serviceMethod string, request interface{}) []byte {
	conn, err := net.DialTimeout("tcp", c.addrs[i], lib.RpcTimeout)
	if err != nil {
		log.Fatalf("Dialing failed: %v", err)
		return nil
	}

	client := rpc.NewClient(conn)
	defer func(client *rpc.Client) {
		if client != nil {
			err := client.Close()
			if err != nil {
				log.Fatalf("Error closing client: %v", err)
			}
		}
	}(client)

	var reply []byte
	err = client.Call(serviceMethod, request, &reply)
	if err != nil {
		log.Fatalf("Error calling %s: %v", serviceMethod, err)
	}
	return reply
}

func (c *Client) CallAll(serviceMethod string, request interface{}) [][]byte {
	replies := make([][]byte, len(c.clients))
	for i, client := range c.clients {
		var reply []byte
		err := client.Call(serviceMethod, request, &reply)
		if err != nil {
			log.Fatalf("Error calling %s: %v", err)
		}
		replies[i] = reply
	}
	return replies
}

func main() {
	addrs := []string{
		"localhost:8080",
		//"localhost:8081",
		//"localhost:8082",
		//"localhost:8083",
		//"localhost:8084",
		//"localhost:8085",
	}
	client := NewClient(addrs)

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
		case "get":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for get")
				continue
			}
			key := parts[1]
			fmt.Println("Getting key", key)
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
		case "request":
			if len(parts) < 2 {
				fmt.Println("Unknown command:", "request "+parts[1])
				continue
			}
			if parts[1] != "log" {
				fmt.Println("Unknown command:", "request "+parts[1])
				continue
			}
			fmt.Println("Requesting log")

			var response []byte
			responses := client.CallAll("RaftNode.RequestLog", "")

			for _, x := range responses {
				if x != nil {
					response = x
					break // assume only leader responds
				}
			}

			if response == nil {
				fmt.Println("No response")
			} else {
				var responseMap map[string][]lib.LogEntry
				err := json.Unmarshal(response, &responseMap)
				if err != nil {
					fmt.Println("Error unmarshalling log entries:", err)
				} else {
					logEntries := responseMap["log"]
					fmt.Println("Log entries:")
					for _, entry := range logEntries {
						fmt.Println(entry)
					}
				}
			}

		default:
			fmt.Println("Unknown command:", cmd)
		}
	}
}
