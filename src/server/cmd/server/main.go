package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)
	
func main() {

	var (
		port  string
		osLen = len(os.Args[1:])
	)

	switch osLen {
	case 0:
		port = "8080"
	case 1:
		// add handling for port overflow (i.e., if it exceeds ~64345 or whatever it is, 
		// or if it uses a lower than whatever the bottom limit is)
		port = os.Args[1]
	default:
		log.Fatal(fmt.Errorf("Error: Invalid number of arguments passed."))
	}

	fmt.Printf("Starting server...\nListening on port %v\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
