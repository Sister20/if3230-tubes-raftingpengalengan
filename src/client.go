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
	client *rpc.Client
	addr   string
}

const (
	maxAttempts = 5
)

func NewClient(addr string) *Client {
	client := &rpc.Client{}

	attempts := 0
	for attempts < maxAttempts {
		log.Printf("Dialing %s", addr)
		conn, err := net.DialTimeout("tcp", addr, lib.RpcTimeout)
		if err != nil {
			log.Printf("Dialing failed: %v", err)
			attempts++
			if attempts == maxAttempts {
				log.Printf("Failed to dial %s after %d attempts", addr, maxAttempts)
				return nil
			}
			continue
		}
		client = rpc.NewClient(conn)
		break
	}
	return &Client{
		client: client,
		addr:   addr,
	}
}

func (c *Client) CallAll(serviceMethod string, request interface{}) []byte {
	var reply []byte
	for {
		err := c.client.Call(serviceMethod, request, &reply)
		if err != nil {
			log.Printf("Error calling %s: %v", serviceMethod, err)
			continue
		}

		var responseMap map[string]any
		err = json.Unmarshal(reply, &responseMap)
		if err != nil {
			log.Printf("Error unmarshalling response: %v", err)
			continue
		}
		if responseMap["leaderAddr"] != nil {
			c.ChangeAddr(responseMap["leaderAddr"].(string))
		} else {
			break
		}
	}

	return reply
}

func (c *Client) ChangeAddr(addr string) {
	err := c.client.Close()
	if err != nil {
		log.Fatalf("Error closing client: %v", err)
	}
	conn, err := net.DialTimeout("tcp", addr, lib.RpcTimeout)
	if err != nil {
		log.Printf("Dialing failed: %v", err)
		return
	}
	if conn == nil {
		log.Printf("Dialing failed: conn is nil")
		return
	}
	c.addr = addr
	c.client = rpc.NewClient(conn)
}

func (c *Client) Execute(cmd string, args string) string {
	var response []byte
	response = c.CallAll("RaftNode.Execute", cmd+" "+args)

	if response == nil {
		return "No response"
	}

	var responseMap map[string]any
	err := json.Unmarshal(response, &responseMap)
	if err != nil {
		return "Error unmarshalling response"
	}

	if responseMap["ok"] != true {
		return "Server error executing command"
	}

	return responseMap["result"].(string)
}

func (c *Client) Ping() bool {
	conn, err := net.DialTimeout("tcp", c.addr, lib.RpcTimeout)
	return err == nil && conn != nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: go run client.go <ip>:<port>")
		os.Exit(0)
	}

	addr := os.Args[1] + ":" + os.Args[2]

	client := NewClient(addr)

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Client started")

	// Use client.Call and client.CallAll to send requests
	for {
		fmt.Print("> ")
		line, _ := reader.ReadString('\n')
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		cmd := parts[0]
		switch cmd {
		case "quit":
			return
		case "ping":
			if client.Ping() {
				fmt.Println("Pong")
			} else {
				fmt.Println("No response")
			}
		case "get":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for get")
				continue
			}
			key := parts[1]
			//fmt.Println("Getting key", key)
			response := client.Execute("get", key)
			fmt.Println("Response:", response)
		case "append":
			if len(parts) < 3 {
				fmt.Println("Not enough arguments for append")
				continue
			}
			key := parts[1]
			value := parts[2]
			//fmt.Println("Appending to key", key, "value", value)
			response := client.Execute("append", key+" "+value)
			fmt.Println("Response:", response)
		case "set":
			if len(parts) < 3 {
				fmt.Println("Not enough arguments for set")
				continue
			}
			key := parts[1]
			value := parts[2]
			//fmt.Println("Setting key", key, "to value", value)
			response := client.Execute("set", key+" "+value)
			fmt.Println("Response:", response)
		case "strlen":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for strlen")
				continue
			}
			key := parts[1]
			//fmt.Println("Getting length of key", key)
			response := client.Execute("strlen", key)
			fmt.Println("Response:", response)
		case "del":
			if len(parts) < 2 {
				fmt.Println("Not enough arguments for del")
				continue
			}
			key := parts[1]
			//fmt.Println("Deleting key", key)
			response := client.Execute("del", key)
			fmt.Println("Response:", response)
		case "request":
			if len(parts) < 2 {
				fmt.Println("Unknown command:", "request "+parts[1])
				continue
			}
			if parts[1] != "log" {
				fmt.Println("Unknown command:", "request "+parts[1])
				continue
			}
			//fmt.Println("Requesting log")

			var response []byte
			response = client.CallAll("RaftNode.RequestLog", "")

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
