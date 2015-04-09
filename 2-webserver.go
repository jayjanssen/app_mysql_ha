package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	writer_ip   string = `192.168.70.100:3306`
	user               = `test`
	pass               = `test`
	schema             = `sakila`
	options            = `timeout=500ms`
	max_idle    int    = 0 // Can't keep idle conns open b/c of: https://github.com/go-sql-driver/mysql/issues/257
	concurrency        = 5 // Keep this script from blasting the database
	retry_delay        = 500 * time.Millisecond
	lag = 100 * time.Millisecond
)

// Our global writer DB handle
var writer *sql.DB
var hits int

func main() {	
	var err error
	// Setup our SQL pool
	writer, err = sql.Open(`mysql`,
		fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", user, pass, writer_ip,
			schema, options))
	if err != nil {
		log.Fatal("Got ", err, "trying to connect to writer")
	}
	writer.SetMaxIdleConns(max_idle)
	writer.SetMaxOpenConns(concurrency)
	defer writer.Close()


	go func() {
		for now := range time.Tick( 1 * time.Second ) {
			if hits > 0 {
				fmt.Println( now.Format(`15:04:05`), hits )
				hits = 0
			}
		}		
	}()
	

	http.HandleFunc("/", handle)
	http.ListenAndServe(":8080", nil)
}

func handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var title string
	for k, v := range r.URL.Query() {
		if k == `title` {
			title = `%` + strings.Join(v, ` `) + `%`
		}
	}

	if title == `` {
		w.WriteHeader(503)
		fmt.Fprintf(w, "Need title\n")
		return
	}

	rows, err := writer.Query(
		`select title, stores, rentals, rental_total  from film_rentals_summary where title like ?`, title)
	if err != nil {
		log.Print("Query error: ", err)
		w.WriteHeader(503)
		fmt.Fprintf(w, "Unable to service request\n")
		return
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var stores, rentals int
		var title string
		var rental_total float64

		err = rows.Scan(&title, &stores, &rentals, &rental_total)
		if err != nil {
			log.Fatal("Scan error: ", err)
		}

		fmt.Fprintf(w, fmt.Sprintf("%s - stores: %d, rentals: %d, total: $%.2f\n", title, stores, rentals, rental_total))
		
		// Add a little lag
		time.Sleep( lag )
		
		count++
	}
	if err = rows.Err(); err != nil {
		log.Print( "Result set fetch failed:", err)
		w.WriteHeader(503)
		fmt.Fprintf(w, "Unable to service request\n")
		return
	}
	
	end := time.Now()

	fmt.Fprintf(w, fmt.Sprintf("Serviced request in %s, got %d results\n", end.Sub(start).String(), count))

	// Log the hit asynchronously
	go log_access( title, start.UTC(), time.Since(start), count)
}

// CREATE TABLE film_rentals_summary_access_log(
//   id bigint unsigned not null auto_increment,
//   search varchar(255) not null,
//   start datetime(3) not null,
//   seconds float not null,
//   count int unsigned not null,
// 	PRIMARY KEY( id )
// );
func log_access(search string, start time.Time, duration time.Duration, count int ) {
	hits++
	// Insert the summarized data into the summary table
	_, err := writer.Exec(`insert into film_rentals_summary_access_log 
		(search, start, seconds, count) values (?, ?, ?, ?)`,
		search, start.Format(`2006-01-02 15:04:05.000`), duration.Seconds(), count)
	if err != nil {
		log.Println("Could not insert into access log: ", err)
	}

}
