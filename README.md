# dockerator
Just some docker orchestrator for fun (partly incompleted). Singlehosted - uses docker:dind as nodes. Just 2 compiled binaries (1 for server, 1 for client)

# Basic system diagram
![alt text](https://github.com/artemantipov/dockerator/blob/master/dockerator.png)

# Thing to do:
* Add more verbose metrics for API server and axpose it by current prometheus endpoint
* Dockerfiles for compile api-server binaries 
* Dockerfile for Nodes based on docker:dind + compiled client binaries
* Compose file to start/build nodes + prometheus + grafana (with embedded dashboard for common metrics of a cluster) + loki (logs)






