package main

import (
	"github.com/gladiusio/gladius-networkd/networkd"
	log "github.com/sirupsen/logrus"
)

// Main - entry-point for the service
func main() {
	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
	networkd.SetupAndRun()
}
