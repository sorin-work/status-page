// Command statuspage runs the uptime monitor and serves the status page.
package main

import (
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stderr, "statuspage: ", log.LstdFlags)
	logger.Println("starting")
}
