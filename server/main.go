package main

import (
	"context"
	pb "dockerator/dockerator"
	kv "dockerator/kvstore"
	"fmt"
	"log"
	"net"
	"strings"
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
	go taskToQueueLoop()
	launchService("BlaBlaBla", "nginx:alpine", 3)
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
	job, params = getTaskFromQueue(node)
	if job != "nojob" {
		fmt.Println(kv.GetKV(db, node))
	}
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

// func nodesCheckLoop() {
// 	for {
// 		runningNodes := docker.NodesHealthChecks()
// 		nodes, _ := kv.GetKV(db, "Nodes")
// 		nodesDesired := strings.Split(nodes, " ")
// 	}
// }

func launchService(name, image string, rs int) (tasks []string) {
	kv.AppendKV(db, "Services", name)
	for i := 0; i < rs; i++ {
		taskName := nameWithSuffix("Task")
		taskParam := fmt.Sprintf("%v %v %v %v", "create", nameWithSuffix(name), image, 1)
		kv.PutKV(db, taskName, taskParam)
		tasks = append(tasks, taskName)
	}
	return tasks
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
		time.Sleep(5 * time.Second)
	}
}

func getTaskFromQueue(node string) (job string, params string) {
	if len(taskQueue) > 0 {
		task := <-taskQueue
		t := strings.Split(task, " ")
		job = t[0]
		params = strings.Join(t[1:], " ")
		contName := t[1]
		contParam := strings.Join(t[2:], " ")
		svcName := contName[:len(contName)-21]
		kv.AppendKV(db, node, contName)
		kv.AppendKV(db, svcName, contName)
		kv.AppendKV(db, contName, contParam)
		return
	}
	log.Println("No task")
	job = "nojob"
	params = "noparams"
	return
}

func nameWithSuffix(name string) (finalName string) {
	// unixTime := fmt.Sprintf("%v", time.Now().UnixNano())
	// finalName = fmt.Sprintf("%v-%v", name, unixTime)
	id := xid.New()
	finalName = fmt.Sprintf("%v-%v", name, id)
	return
}
