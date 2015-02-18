<?php
	
// Connecting, selecting database
$mysqli = new mysqli("localhost", "test", "test", "imdb");
if ($mysqli->connect_errno) {
    echo "Failed to connect to MySQL: (" . $mysqli->connect_errno . ") " . $mysqli->connect_error;
}
echo "Connected to: " . $mysqli->host_info . "\n";

$result = $mysqli->query('SELECT * FROM title', MYSQLI_USE_RESULT);
	
// Printing results
$count = 0;
while( $row = $result->fetch_assoc() ){
	echo $row['title'];
	echo "\n";
	$count++;

	if( $count >= 1000 ) {
		sleep( 1 );
		$count = 0;
	}
}

if( $mysqli->sqlstate != '00000' ) {
	die( "Fetch failed, got error: " . $mysqli->sqlstate . "\n" );
}

// Closing connection
$mysqli->close();

?>