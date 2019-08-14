package main

import (
	"context"
	pb "dockerator/dockerator"
	kv "dockerator/kvstore"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/rs/xid"
	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

var db = kv.InitDB("/tmp/db")
var taskQueue = make(chan string, 100)

type server struct{}

func main() {
	defer db.Close()
	go grpcServerStart()
	for {
		time.Sleep(5 * time.Second)
	}

}

func (s *server) CheckWorker(ctx context.Context, request *pb.Request) (*pb.Response, error) {
	node, service, state := request.GetNode(), request.GetService(), request.GetState()
	command, params, status := checkByNode(node, service, state)
	return &pb.Response{Command: command, Params: params, Status: status}, nil
}

func (s *server) CheckForTask(ctx context.Context, request *pb.TaskRequest) (*pb.TaskResponse, error) {
	node := request.GetNode()
	job, params := checkForTask(node)
	return &pb.TaskResponse{Job: job, Params: params}, nil
}

func checkByNode(node string, service string, state string) (command string, params string, status bool) {
	log.Printf("Received message from %v", node)
	command = "NoCommand"
	params = fmt.Sprintf("ACK for %v", node)
	status = true
	if state != "running" {
		command = "recreate"
		params = fmt.Sprintf("%v nginx:alpine", service)
		status = false
	}

	if service == "nodereg" {
		kv.AppendKV(db, "Nodes", node)
		fmt.Println(kv.GetKV(db, "Nodes"))
		params = "Node Registered"
	}
	return
}

func checkForTask(node string) (job string, params string) {
	job = "nojob"
	params = "noparams"
	return
}

func grpcServerStart() {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterDockeratorServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func launchService(name, image string, rs int) {
	kv.AppendKV(db, "Services", name)
	taskParam := fmt.Sprintf("%v %v %v %v", "create", nameWithSuffix(name), image, 1)
	for i := 0; i < rs; i++ {
		kv.PutKV(db, nameWithSuffix("Task"), taskParam)
	}
}

func taskToQueueLoop() {
	for {
		tasks := kv.TasksList(db)
		if len(tasks) != 0 {
			for _, v := range tasks {
				task, _ := kv.GetKV(db, v)
				taskQueue <- task
				kv.DeleteKV(db, v)
			}
		}
	}
}

func nameWithSuffix(name string) (finalName string) {
	// unixTime := fmt.Sprintf("%v", time.Now().UnixNano())
	// finalName = fmt.Sprintf("%v-%v", name, unixTime)
	id := xid.New()
	finalName = fmt.Sprintf("%v-%v", name, id)
	return
}
