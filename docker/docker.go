package docker

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/rs/xid"
	"golang.org/x/net/context"
)

var ctx = context.Background()

func dockerCli() (cli *client.Client) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Println(err)
	}
	return
}

// Container operations
func Container(action string, params string) {
	cli := dockerCli()
	args := strings.Split(params, " ")
	switch action {
	case "create":
		imageName := args[1]
		replicas := 1
		if len(args) > 2 {
			rs, err := strconv.Atoi(args[2])
			if err == nil {
				replicas = rs
			}
		}
		if ImageExist(imageName) != true {
			out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
			if err != nil {
				log.Println(err)
			}
			io.Copy(os.Stdout, out)
		}
		for i := 0; i < replicas; i++ {
			contName := nameWithSuffix(args[0])
			resp, err := cli.ContainerCreate(ctx, &container.Config{
				Image: imageName,
			}, nil, nil, contName)
			if err != nil {
				log.Println(err)
			}
			if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
				log.Println(err)
			}
			fmt.Println(resp.ID)
		}
	case "recreate":
		oldContName := args[0]
		imageName := args[1]
		replicas := 1
		if len(args) > 2 {
			rs, err := strconv.Atoi(args[2])
			if err == nil {
				replicas = rs
			}
		}
		if err := cli.ContainerRemove(ctx, GetContID(oldContName), types.ContainerRemoveOptions{}); err != nil {
			log.Println(err)
		}
		for i := 0; i < replicas; i++ {
			contName := nameWithSuffix(oldContName[:len(oldContName)-21])
			resp, err := cli.ContainerCreate(ctx, &container.Config{
				Image: imageName,
			}, nil, nil, contName)
			if err != nil {
				log.Println(err)
			}
			if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
				log.Println(err)
			}
			fmt.Println(resp.ID)
		}
	case "delete":
		containerName := args[0]
		if err := cli.ContainerStop(ctx, GetContID(containerName), nil); err != nil {
			log.Println(err)
		}
		if err := cli.ContainerRemove(ctx, GetContID(containerName), types.ContainerRemoveOptions{}); err != nil {
			log.Println(err)
		}
	default:
		fmt.Println("Wrong action")
	}
}

// GetContID return ID of requested container
func GetContID(name string) (ID string) {
	cli := dockerCli()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		log.Println(err)
	}
	for _, container := range containers {
		containerName := strings.TrimLeft(container.Names[0], "/")
		if containerName == name {
			ID = container.ID
		}
	}
	return
}

// CheckContainer - return exact container state
func CheckContainer(name string) {
	cli := dockerCli()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		log.Println(err)
	}

	for _, container := range containers {
		containerName := strings.TrimLeft(container.Names[0], "/")
		if containerName == name {
			fmt.Printf("Container: %s\nStatus: %s\nUptime: %s\n\n", containerName, container.State, container.Status)
		}
	}
}

// CheckService - return service state
func CheckService(args ...string) (result bool) {
	name := args[0]
	cli := dockerCli()
	replicas := 1
	if len(args) > 1 {
		rs, err := strconv.Atoi(args[1])
		if err == nil {
			replicas = rs
		}
	}
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		log.Println(err)
	}
	rsCheck := 0
	for _, container := range containers {
		containerName := strings.TrimLeft(container.Names[0], "/")
		if strings.HasPrefix(containerName, name) {
			if container.State == "running" {
				rsCheck++
			}
			fmt.Printf("Service: %s\nStatus: %s\nUptime: %s\n\n", containerName, container.State, container.Status)
		}
	}
	if rsCheck != replicas {
		return false
	}
	return true
}

// GetContainerIP - return container IP by ID
func GetContainerIP(name string) (IP string) {
	cli := dockerCli()
	container, err := cli.ContainerInspect(ctx, GetContID(name))
	if err != nil {
		log.Println(err)
	}
	IP = container.NetworkSettings.IPAddress
	return
}

// // AddAgent - deploy agent to node
// func AddAgent() {
// 	cli := dockerCli()
// 	imageName := "nginx:alpine"
// 	out, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	io.Copy(os.Stdout, out)
// 	config := &container.Config{
// 		Image: imageName,
// 		ExposedPorts: nat.PortSet{
// 			"80/tcp": {}},
// 	}
// 	hostConfig := &container.HostConfig{
// 		PortBindings: nat.PortMap{
// 			"80/tcp": []nat.PortBinding{
// 				{
// 					HostIP:   "0.0.0.0",
// 					HostPort: "8080",
// 				},
// 			},
// 		},
// 	}
// 	resp, err := cli.ContainerCreate(ctx, config, hostConfig, nil, "dockerator-agent")
// 	if err != nil {
// 		log.Println(err)
// 	}
// 	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
// 		log.Println(err)
// 	}
// }

// PS - listing of all containers
func PS(param string) (containers []types.Container) {
	cli := dockerCli()
	switch param {
	case "all":
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
		if err != nil {
			log.Println(err)
		}
		// for _, container := range containers {
		// 	fmt.Printf("%s %s %s\n", container.ID[:10], container.Image, container.State)
		// 	fmt.Printf("%+v\n", container)
		// }
		return containers
	case "up":
		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			log.Println(err)
		}
		// for _, container := range containers {
		// 	fmt.Printf("%s %s %s\n", container.ID[:10], container.Image, container.State)
		// 	fmt.Printf("%+v\n", container)
		// }
		return containers
	default:
		fmt.Println("param must be all|up")
	}
	return
}

// NodesHealthChecks - status of "nodes"
func NodesHealthChecks() {
	cli := dockerCli()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		log.Println(err)
	}
	for _, container := range containers {
		nodeName := strings.TrimLeft(container.Names[0], "/")
		if container.State == "running" {
			fmt.Printf("%v node is UP\n", nodeName)
		} else {
			fmt.Printf("%v node is DOWN\n", nodeName)
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

// NodeToDeploy - select node to deploy
func NodeToDeploy() (nodeCandidate string) {
	cli := dockerCli()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Println(err)
	}
	nodes := []string{}
	for _, container := range containers {
		nodeName := strings.TrimLeft(container.Names[0], "/")
		if strings.HasPrefix(nodeName, "node") {
			nodes = append(nodes, nodeName)
		}
	}
	nodeCandidate = fmt.Sprintf("%v", nodes[randomSelect(3)])
	return
}

func randomSelect(limitID int) (randID int) {
	rand.Seed(time.Now().UnixNano())
	randID = rand.Perm(limitID)[0]
	return
}

// GetNodeIP - return ip of node(cotainer)
func GetNodeIP(iface string) (ip string) {
	interfaces, _ := net.Interfaces()
	for _, i := range interfaces {
		if i.Name == iface {
			addrs, _ := i.Addrs()
			ip = strings.Split(fmt.Sprintf("%s", addrs[0]), "/")[0]
		}
	}
	return
}

// ImageExist - check image by name
func ImageExist(imageName string) (result bool) {
	cli := dockerCli()

	images, err := cli.ImageList(context.Background(), types.ImageListOptions{})
	if err != nil {
		panic(err)
	}
	result = false
	for _, image := range images {
		for _, tag := range image.RepoTags {
			if tag == imageName {
				result = true
			}
		}
	}
	return
}
