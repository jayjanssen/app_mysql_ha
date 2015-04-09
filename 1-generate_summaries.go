package main

import (
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
  "time"
  "fmt"
)

// Our global writer DB handle
var writer *sql.DB
const (
  writer_ip string = `192.168.70.100:3306`
  user = `test`
  pass = `test`
  schema = `sakila`
  lag = 5 * time.Millisecond
)

func main() {
  // Setup our SQL pool
	writer, _ = sql.Open(`mysql`, 
    fmt.Sprintf( "%s:%s@tcp(%s)/%s?%s", user, pass, writer_ip, 
      schema))
  
  // Truncate the table at the beginning
  writer.Exec( `truncate table film_rentals_summary`)

  // Get the films to calculate  
	rows, _ := writer.Query( 
    `select film_id from film order by film_id asc`)

  var rows_fetched int
  for rows.Next() {      
    var film_id int
    rows.Scan( &film_id )

    fmt.Println( "Processing film", film_id )

    var (
      title string
      store_count, rental_count int
      rental_total float64
    )
    writer.QueryRow( 
      `select title, count( distinct store_id) as stores, count( distinct rental_id) as count, 
          ifnull( sum(amount), 0.0 ) as total_payments 
        from film left join inventory using( film_id ) 
          left join rental using( inventory_id ) 
          left join payment using( rental_id ) 
        where film_id=?
        group by film_id`, film_id).Scan(&title, &store_count, 
          &rental_count, &rental_total)
    
    // Inject some application TX processing lag
    time.Sleep( lag )
    
    // Insert the summarized data into the summary table
    writer.Exec(`insert into film_rentals_summary 
      (film_id, title, stores, rentals, rental_total) 
      values (?, ?, ?, ?, ?)
      on duplicate key update title=values(title), 
        rentals=values(rentals), rental_total=values(rental_total)`, 
      film_id, title, store_count, rental_count, rental_total)  
    
    rows_fetched++
  }
  fmt.Println( "Done, fetched", rows_fetched )
}