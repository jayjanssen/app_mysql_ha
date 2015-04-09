package main

import (
	_ "github.com/go-sql-driver/mysql"
	"database/sql"
	"fmt"
  "log"
  "time"
)

// Our global writer DB handle
var writer *sql.DB
const (
  writer_ip string = `192.168.70.100:3306`
  user = `test`
  pass = `test`
  schema = `sakila`
  options = `timeout=10s&wait_timeout=60`
  max_idle int = 0  // Can't keep idle conns open b/c of: https://github.com/go-sql-driver/mysql/issues/257
  concurrency = 5 // Keep this script from blasting the database
  retry_delay = 500 * time.Millisecond
  lag = 500 * time.Millisecond
)

func main() {
  var err error
  // Setup our SQL pool
	writer, err = sql.Open(`mysql`, 
    fmt.Sprintf( "%s:%s@tcp(%s)/%s?%s", user, pass, writer_ip, 
      schema, options ))
	if err != nil {
		log.Fatal( "Got ", err, "trying to connect to writer")
	}
  writer.SetMaxIdleConns(max_idle)
  writer.SetMaxOpenConns(concurrency)
	defer writer.Close()
  
  // Truncate the table at the beginning
  for {
    _, err = writer.Exec( `truncate table film_rentals_summary`)
    if err != nil {
      log.Print(`Could not truncate: `, err)
      time.Sleep( retry_delay )
    } else {
      break;
    }
  }
  
  last_id := 0
  for {
    ch := make( chan int )
    start := time.Now()
  	rows, err := writer.Query( 
      `select film_id from film where film_id > ? 
        order by film_id asc limit ?`, last_id, concurrency)
    if err != nil {
      log.Print( "Query error: ", err )
      time.Sleep( retry_delay )
      continue;
    }
    defer rows.Close()
    log.Print( "query ", last_id, " ", time.Since( start ).String() )
  
    var rows_fetched int
    for rows.Next() {      
      var film_id int
      err = rows.Scan( &film_id )
      if err != nil {
        log.Fatal( "Scan error: ", err )
      }
      
      go process_film( film_id, ch )
      
      last_id = film_id
      rows_fetched++
    }
  	if err = rows.Err(); err != nil {
  		log.Print( "Result set fetch failed:", err)
  	} else if rows_fetched == 0 {
      break // No errors and 0 rows fetched means we're done
    } else {
      // Wait for rows_fetched signals from ch to know that the last batch is done
      for i := 0; i < rows_fetched; i++ {
        <- ch
      }      
    }
  }
  fmt.Println( "Done")
}

//
// mha3 mysql> select count(*) from sakila.film_rentals_summary;
// +----------+
// | count(*) |
// +----------+
// |      958 |
// +----------+
// 1 row in set (0.00 sec)
//
// mha3 mysql> pager md5sum                                                                               PAGER set to 'md5sum'
// mha3 mysql> select * from film_rentals_summary;                                                        92814c7e39d4e8dc1c4a5fae88378a5f  -
// 958 rows in set (0.00 sec)
//

// process_film generates a summary of sales by store for the given film_id and inserts the results in to the following table:
// CREATE TABLE film_rentals_summary(
//   film_id smallint(5) unsigned not null,
//   title varchar(255) not null,
//   stores int unsigned not null,
//   rentals int unsigned not null,
//   rental_total decimal(5,2) not null,
//   PRIMARY KEY( film_id ),
//   KEY( title(20) )
// );
func process_film( film_id int, ch chan int ) {
  // Setup variables that we can refer to in our closure
  var trx *sql.Tx
  var err error
  
  // Closure to wrap standard rollback and retry procedure
  rollback_and_try_again := func(msg string) {
    trx.Rollback()
    log.Print( msg, err )
    time.Sleep( retry_delay )
    go process_film( film_id, ch )
  }
  
  // BEGIN transaction
  trx, err = writer.Begin()
  if err != nil {
    rollback_and_try_again( "Could not begin trx: " )
    return
  }
  
  // Get the data for our film first
  var (
    title string
    store_count, rental_count int
    rental_total float64
  )
	err = trx.QueryRow( 
    `select title, count( distinct store_id) as stores, count( distinct rental_id) as count, 
        ifnull( sum(amount), 0.0 ) as total_payments 
      from film left join inventory using( film_id ) 
        left join rental using( inventory_id ) 
        left join payment using( rental_id ) 
      where film_id=?
      group by film_id`, film_id).Scan(&title, &store_count, 
        &rental_count, &rental_total)
  switch {
  case err == sql.ErrNoRows:
    // No results, we don't do anything here.
    trx.Rollback()
    ch <- 1
    return
  case err != nil:
    rollback_and_try_again( "Got query error: " )
    return
  }
  
  // Inject some application TX processing lag
  time.Sleep( lag )
  
  // Insert the summarized data into the summary table
  _, err = trx.Exec(`insert into film_rentals_summary 
    (film_id, title, stores, rentals, rental_total) 
    values (?, ?, ?, ?, ?)
    on duplicate key update title=values(title), 
      rentals=values(rentals), rental_total=values(rental_total)`, 
    film_id, title, store_count, rental_count, rental_total)
  if err != nil {
    rollback_and_try_again( "Could not insert: " )
    return
  }
  
  // Commit the trx
  err = trx.Commit()
  if err != nil {
    rollback_and_try_again( "Got error on commit: " )
  } else {
    ch <- 1 // tell them we're done!
  }
}
