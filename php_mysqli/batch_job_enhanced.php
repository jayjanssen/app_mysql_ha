<?php

$first = true;
$last_id = NULL;
do {
	if( !$first ) {
		// Reconnection
		sleep( 1 );
		echo "Got error", $mysqli->sqlstate, ", reconnecting\n";
	}

	// Connecting, selecting database
	$mysqli = new mysqli("localhost", "test", "test", "imdb");
	if ($mysqli->connect_errno) {
	    echo "Failed to connect to MySQL: (" . $mysqli->connect_errno . ") " . $mysqli->connect_error . "\n";
			continue;
	}
	echo "Connected to: " . $mysqli->host_info . "\n";

	$query = 'SELECT * FROM title';
	if( isset( $last_id )) {
		echo "Left off on $last_id, resuming from there\n";
		$query .= " WHERE id > $last_id";
	}
		
	// Printing results
	$count = 0;
	foreach( $mysqli->query($query, MYSQLI_USE_RESULT) as $row ){
		$last_id = $row['id'];  // Keep track of where we are in case we have to start over
		$count++;

		if( $count >= 1000 ) {
			$count = 0;
			print "$last_id\n";
			sleep( 1 );
		}
	}
	$first = false;
} while( $mysqli->sqlstate != '00000' );

echo 'Final state: ', $mysqli->sqlstate, "\n";

// Closing connection
$mysqli->close();

?>