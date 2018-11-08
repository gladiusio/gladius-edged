package main

import (
	"github.com/gladiusio/gladius-edged/networkd"
)

// Main - entry-point for the service
func main() {
	networkd.SetupAndRun()
}
