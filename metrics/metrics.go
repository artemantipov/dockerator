package metrics

import (
	"log"
	"os"
	"strings"
	"io/ioutil"
	"time"
	"fmt"

	"github.com/labstack/echo"
)
// Start metric handler endpoint
func Start(queue chan string) {
	go workerWriter(queue)
	e := echo.New()
	e.GET("/metrics", func(c echo.Context) error {
		return c.File("/metrics")
	})
	e.Logger.Fatal(e.Start(":1221"))
}

func workerWriter(queue chan string) {
	fmt.Println("worker started")
	for {
		for i := len(queue); i > 0; i = len(queue) {
			metric := <-queue
			writeMetric(metric)
		}
		time.Sleep(60 * time.Second)
	}
}

func writeMetric(metric string) {
	metric = strings.ToLower(metric)
	if !fileExists("/metrics") {
		file, err := os.Create("/metrics")
		if err != nil {
			log.Printf("Can't create a file: %v", err)
		}
		file.Close()
	} 

	input, err := ioutil.ReadFile("/metrics")
	if err != nil {
		log.Fatalln(err)
	}
	lines := strings.Split(string(input), "\n")
	pair := strings.Split(metric, " ")
	metricExist := false
	for i, line := range lines {
		if strings.Contains(line, fmt.Sprintf("%v ",pair[0])) {
			lines[i] = metric
			metricExist = true
		}
	}
	if metricExist != true {
		lines = append(lines, metric)
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile("/metrics", []byte(output), 0666)
	if err != nil {
		log.Fatalln(err)
	}
}

// WriteToChannel add metrics to queue
func WriteToChannel(metrics []string, queue chan string) {
	for _, metric := range metrics {
		mt := strings.ToLower(metric)
		queue <- mt
	}
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
