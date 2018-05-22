package networkd

import (
	"fmt"

	"github.com/gladiusio/gladius-networkd/networkd/server/contserver"
	"github.com/gladiusio/gladius-networkd/networkd/server/rpcserver"
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
	s := state.New()

	// Create a content server
	cs := contserver.New(s)
	defer cs.Stop()

	// Create an rpc server
	rpc := rpcserver.New(s)
	defer rpc.Stop()

	fmt.Println("Started RPC server and HTTP server.")

	// Forever check through the channels on the main thread
	for {
		select {
		case runState := <-s.RunningStateChanged(): // If it can be assigned to a variable
			if runState {
				cs.Start()
			} else {
				cs.Stop()
			}
		}
	}
}
