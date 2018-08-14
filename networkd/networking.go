package networkd

import (
	"fmt"

	"github.com/gladiusio/gladius-networkd/networkd/p2p/handler"
	"github.com/gladiusio/gladius-networkd/networkd/server/contserver"
	"github.com/gladiusio/gladius-networkd/networkd/state"

	"github.com/gladiusio/gladius-utils/config"
	"github.com/gladiusio/gladius-utils/init/manager"
)

// SetupAndRun runs the networkd as a service
func SetupAndRun() {
	// Define some variables
	name, displayName, description :=
		"GladiusNetworkDaemon",
		"Gladius Network (Edge) Daemon",
		"Gladius Network (Edge) Daemon"

	// Run the function "run" in newtworkd as a service
	manager.RunService(name, displayName, description, Run)
}

// Run - Start a web server
func Run() {
	fmt.Println("Loading config")

	// Setup config handling
	config.SetupConfig("gladius-networkd", config.NetworkDaemonDefaults())

	fmt.Println("Starting...")

	// Create new thread safe state of the networkd
	s := state.New("0.3.0")

	// Create a content server
	cs := contserver.New(s)
	cs.Start()
	defer cs.Stop()

	// Create a p2p handler
	controldBase := config.GetString("ControldProtocol") + "://" + config.GetString("ControldHostname") + ":" + config.GetString("ControldPort") + "/api/p2p"
	// TODO: Get seed node from the blockchain
	p2pHandler := handler.New(controldBase, config.GetString("P2PSeedNodeAddress"))
	p2pHandler.Connect()

	fmt.Println("Started HTTP server.")

	// Block forever
	select {}
}
