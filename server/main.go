package main

import (
	"context"
	"dockerator/docker"
	pb "dockerator/dockerator"
	kv "dockerator/kvstore"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/xid"
	"google.golang.org/grpc"
)

const (
	port = ":50051"
)

var db = kv.InitDB("/tmp/db")
var taskQueue = make(chan string, 100)

type server struct{}

type svcConfig struct {
	Name     string `json:"name"`
	Image    string `json:"image"`
	Replicas int    `json:"rs"`
}

type node struct {
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Uptime string `json:"uptime"`
}

type service struct {
	Name       string      `json:"name"`
	Replicas   int         `json:"rs"`
	Containers []container `json:"containers"`
}

type container struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Node  string `json:"node"`
	// Uptime string `json:"uptime"`
}

type stateResponse struct {
	Nodes    []node    `json:"nodes"`
	Services []service `json:"services"`
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Test App for P")
}

func svc(c echo.Context) error {
	service := svcConfig{}
	err := c.Bind(&service)
	if err != nil {
		log.Printf("Failed to decode json: %v", err)
		launchService(service.Name, service.Image, service.Replicas)
		return c.String(http.StatusInternalServerError, "Wrong JSON format")
	}

	return c.JSON(http.StatusOK, service)
}

func state(c echo.Context) error {
	nodes := []node{}
	services := []service{}
	nodesMap := docker.GetNodeMap()

	allNodes, _ := kv.GetKV(db, "Nodes")
	for _, n := range strings.Split(allNodes, " ") {
		name := nodesMap[n]
		node := node{name, docker.GetContainerIP(name), docker.GetContainerUptime(name)}
		nodes = append(nodes, node)
	}

	allServices, _ := kv.GetKV(db, "Services")
	for _, s := range strings.Split(allServices, " ") {
		rs := kv.CountRS(db, s)
		containers := []container{}
		containersList, _ := kv.GetKV(db, s)
		for _, c := range strings.Split(containersList, " ") {
			i, _ := kv.GetKV(db, c)
			image := strings.Split(i, " ")[0]
			node := ""
			for k, v := range nodesMap {
				conts, _ := kv.GetKV(db, k)
				if strings.Contains(conts, c) {
					node = v
				}
			}
			container := container{c, image, node}
			containers = append(containers, container)
		}
		service := service{s, rs, containers}
		services = append(services, service)
	}
	resp := stateResponse{nodes, services}
	return c.JSON(http.StatusOK, resp)
}

func main() {
	defer db.Close()
	go grpcServerStart()
	go taskToQueueLoop()
	go nodesCheckLoop()

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.GET("/", hello)
	e.POST("/service", svc)
	e.GET("/state", state)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))

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
		oldContName := service
		contName := nameWithSuffix(oldContName[:len(oldContName)-21])
		kv.DeleteKV(db, oldContName)
		kv.EjectKV(db, node, oldContName)
		kv.AppendKV(db, node, contName)
		command = "recreate"
		params = fmt.Sprintf("%v %v nginx:alpine", oldContName, contName)
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

func nodesCheckLoop() {
	for {
		runningNodes := docker.NodesHealthChecks()
		nodes, _ := kv.GetKV(db, "Nodes")
		nodesDesired := strings.Split(nodes, " ")
		nodesChecked := []string{}
		for _, nd := range nodesDesired {
			for _, nr := range runningNodes {
				if nr == nd {
					nodesChecked = append(nodesChecked, nr)
				}
			}
		}
		if len(nodesChecked) != len(nodesDesired) {
			log.Println("Some node failed! Rebalancing...")
		}
		time.Sleep(3 * time.Second)
	}
}

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

func rebalanceService(node, oldSvcName, image string, rs int) {
	kv.DeleteKV(db, oldSvcName)
	kv.EjectKV(db, node, oldSvcName)
	svcName := oldSvcName[:len(oldSvcName)-21]
	for i := 0; i < rs; i++ {
		taskName := nameWithSuffix("Task")
		taskParam := fmt.Sprintf("%v %v %v %v", "recreate", nameWithSuffix(svcName), image, 1)
		kv.PutKV(db, taskName, taskParam)
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
		kv.AppendKV(db, "Services", svcName)
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
	id := xid.New()
	finalName = fmt.Sprintf("%v-%v", name, id)
	return
}
