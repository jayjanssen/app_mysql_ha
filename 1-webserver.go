package main

import (
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"net/http"
	"strings"
	"time"
)

const (
	writer_ip   string = `192.168.70.100:3306`
	user               = `test`
	pass               = `test`
	schema             = `sakila`
	lag = 100 * time.Millisecond
)

// Our global writer DB handle
var writer *sql.DB
var hits int

func main() {	
	// Setup our SQL pool
	writer, _ = sql.Open(`mysql`,
		fmt.Sprintf("%s:%s@tcp(%s)/%s?%s", user, pass, writer_ip, schema))

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

	rows, _ := writer.Query(
		`select title, stores, rentals, rental_total  from film_rentals_summary where title like ?`, title)

	var count int
	for rows.Next() {
		var stores, rentals int
		var title string
		var rental_total float64

		rows.Scan(&title, &stores, &rentals, &rental_total)
		
		// Add a little lag
		time.Sleep( lag )
		fmt.Fprintf(w, fmt.Sprintf("%s - stores: %d, rentals: %d, total: $%.2f\n", title, stores, rentals, rental_total))

		count++
	}
	
	end := time.Now()

	fmt.Fprintf(w, fmt.Sprintf("Serviced request in %s, got %d results\n", end.Sub(start).String(), count))

	hits++
	// Insert the summarized data into the summary table
	writer.Exec(`insert into film_rentals_summary_access_log 
		(search, start, seconds, count) values (?, ?, ?, ?)`,
		title, start.Format(`2006-01-02 15:04:05.000`), end.Sub(start).Seconds(), count)
	time.Sleep( lag )

}
