package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"dockerator/docker"
	pb "dockerator/dockerator"

	"google.golang.org/grpc"
)

const (
	address = "172.17.0.1:50051"
)

var node = docker.GetNodeIP("eth0")

func main() {
	nodeRegister()
	go checkContainersLoop()
	go checkForTaskLoop()
	for {
		time.Sleep(5 * time.Second)
	}
}

func sendMessage(msgType string, args ...string) {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewDockeratorClient(conn)

	// Contact the server and print out its response.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	switch msgType {
	case "check":
		node, service, state := args[0], args[1], args[2]
		r, err := c.CheckWorker(ctx, &pb.Request{Node: node, Service: service, State: state})
		if err != nil {
			log.Fatalf("could not check: %v", err)
		}
		log.Printf("CheckRespond: %v %v %v", r.Command, r.Params, r.Status)
		if r.Status != true {
			log.Printf("Fix service: %v %v\n", r.Command, r.Params)
			docker.Container(r.Command, r.Params)
		}
	case "task":
		node := args[0]
		r, err := c.CheckForTask(ctx, &pb.TaskRequest{Node: node})
		if err != nil {
			log.Fatalf("could not check: %v", err)
		}
		log.Printf("TaskRespond: %v %v ", r.Job, r.Params)
		if r.Job != "nojob" {
			log.Printf("Doing task: %v %v\n", r.Job, r.Params)
			docker.Container(r.Job, r.Params)
		}
	default:
		log.Println("wrong msgType.")
	}
}

func checkContainersLoop() {
	for {
		containers := docker.PS("all")
		for _, container := range containers {
			fmt.Printf("%v - %v - %v\n", node, strings.TrimLeft(container.Names[0], "/"), container.State)
			sendMessage("check", node, strings.TrimLeft(container.Names[0], "/"), container.State)
		}
		time.Sleep(5 * time.Second)
	}
}

func nodeRegister() {
	sendMessage("check", node, "nodereg", "running")
}

func checkForTaskLoop() {
	for {
		sendMessage("task", node)
		time.Sleep(5 * time.Second)
	}
}
