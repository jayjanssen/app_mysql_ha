package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"
	"io/ioutil"
)

const (
	concurrency int    = 3
	url         string = `http://127.0.0.1:8080?title=`
	retry_delay        = 500 * time.Millisecond
)

var queries = []string{
	`dino`,
	`alien`,
	`blues`,
	`citizen`,
	`dolls`,
	`easy`,
	`falcon`,
	`greed`,
	`horn`,
	`miracle`,
	`odds`,
	`road`,
	`sabrina`,
	`stone`,
	`taxi`,
	`whisperer`,
	`youth`,
}

var hits int
var rtts []time.Duration
var success = map[int]int{}
var failed = map[int]int{}


// Create a single client (connection pool)
var client = &http.Client{
	Transport: &http.Transport{
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 1000 * time.Millisecond,
	}}

func main() {
	rand.Seed(time.Now().Unix())
	
	
	go func() {
		for now := range time.Tick( 1 * time.Second ) {
			if hits > 0 {
				fmt.Print( now.Format(`15:04:05`), " ", hits )
				
				var total, count float64
				for _, rtt := range rtts {
					total += rtt.Seconds()
					count++
				}
				rtts = nil
				
				fmt.Print( " ", fmt.Sprintf( "%.3f", total / count ))
				
				for i := 0; i < concurrency; i++ {
					fmt.Print( " ", i, ":", success[i], "/", failed[i] )
					success[i], failed[i] = 0, 0
				}
				fmt.Println()
				hits = 0
			}
		}		
	}()

	// Channel of requests
	requests := make(chan string, 32)

	for i := 0; i < concurrency; i++ {
		go run_worker(i, requests)
	}

	for {
		requests <- queries[rand.Intn(len(queries))]
	}
}

func run_worker(id int, requests chan string) {
	for request := range requests {
		start := time.Now()
		res, err := client.Get(url + request)

		var body []byte
		if err == nil {
			defer res.Body.Close()
			body, _ = ioutil.ReadAll(res.Body)
		}
		end := time.Now()
		rtts = append( rtts, end.Sub(start))
		hits++

		if err == nil && res.StatusCode == 200 && len(body) > 0  {
			success[id]++
		} else {
			var code int
			if res != nil {
				code = res.StatusCode
			}
			fmt.Println( id, "had problem, err:", err, "body len:", len(body), "StatusCode:", code )
			time.Sleep( retry_delay )
			failed[id]++
		}
	}
}
