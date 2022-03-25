package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const reqTimeout = 10

var concurrency, _ = strconv.Atoi(os.Getenv("CONCURRENCY"))
var totalReqCount, _ = strconv.Atoi(os.Getenv("REQ_COUNT"))

var hist = []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 110, 120, 130, 140, 150, 160, 170, 180, 190, 200, 250, 300, 400, 500, 600, 700, 800, 900, 1000, 1250, 1500, 1750, 2000, 2250, 2500, 2750, 3000, 3500, 4000, 4500, 5000, 6000, 7000, 8000, 9000}

type Job struct {
	url     string
	method  string
	payload string
}

type Response struct {
	statusCode int
	duration   int
	length     int
	url        string
}

func getRequest(url string) (int, int, int) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	startTime := time.Now()
	resp, err := client.Get(url)
	deltaDuration := time.Now().Sub(startTime)

	if err != nil {
		return 0, int(deltaDuration.Milliseconds()), 0
	}
	defer resp.Body.Close()

	scanner, _ := ioutil.ReadAll(resp.Body)
	strLen := len(scanner)

	return resp.StatusCode, int(deltaDuration.Milliseconds()), strLen
}

func getUrls() *[]string {
	var urls []string

	file, err := os.Open("urls.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		urls = append(urls, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return &urls
}

func histMapKey(duration int) int {
	for _, threshold := range hist {
		if duration <= threshold {
			return threshold
		}
	}
	return 1000 * reqTimeout
}

func main() {
	rand.Seed(time.Now().UnixNano())
	var wg sync.WaitGroup
	numJobs := 0

	if concurrency == 0 {
		concurrency = 3
	}
	if totalReqCount == 0 {
		totalReqCount = 10
	}
	fmt.Printf("Total Requests: %d - Concurrency: %d\n", totalReqCount, concurrency)

	jobs := make(chan Job)
	results := make(chan Response, concurrency)

	http.DefaultTransport.(*http.Transport).ResponseHeaderTimeout = time.Second * reqTimeout

	wg.Add(concurrency)

	for i := 1; i <= concurrency; i++ {
		go func(id int) {
			defer wg.Done()

			for job := range jobs {
				statusCode, duration, bodyLen := getRequest(job.url)

				results <- Response{statusCode, duration, bodyLen, job.url}
			}
		}(i)
	}

	durationHist := make(map[int]int)
	statusMap := make(map[int]int)

	wg.Add(1)
	go func() {
		defer wg.Done()
		var iJob int
		var totalDuration int

		for response := range results {
			fmt.Printf("Status: \t%d\t in \t%6d\t milisec - Size: \t%.2f\t kb - URL: \t%s\n", response.statusCode, response.duration, float32(response.length)/1024, response.url)

			durationHist[histMapKey(response.duration)]++
			statusMap[response.statusCode]++
			totalDuration += response.duration

			iJob++
			if iJob == numJobs {
				fmt.Println("Time ms     Req")
				for _, histKey := range hist {
					val, ok := durationHist[histKey]
					if ok {
						percentage := float32(val) / float32(numJobs) * 100
						histView := strings.Repeat("#", int(percentage))
						fmt.Printf("%6d: %6d  -  %5.2f%%  -  %s\n", histKey, val, percentage, histView)
					}
				}

				rps := float32(numJobs) / float32(totalDuration) * 1000 * float32(concurrency)

				fmt.Print("\nStatus:", statusMap, "\n\n")
				fmt.Printf("Average duration per request: %.2f ms\n\n", float32(totalDuration)/float32(numJobs))
				fmt.Printf("Rate: %.1f req/s ~ %.0f req/min\n\n", rps, rps*60)
				fmt.Printf("Total duration (avg): %.2f sec\n\n", (float32(totalDuration)/1000)/float32(concurrency))
				break
			}
		}
	}()

	urls := *getUrls()
	lenUrls := len(urls)

	for {
		jobs <- Job{url: urls[numJobs%lenUrls]}
		numJobs++

		if numJobs == totalReqCount {
			break
		}
	}

	close(jobs)
	wg.Wait()
}
