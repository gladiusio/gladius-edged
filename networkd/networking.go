package networkd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/buger/jsonparser"
	"github.com/rdegges/go-ipify"

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
	s := state.New("0.3.0")

	// Start up p2p connection
	go connectToP2P(config.GetString("P2PSeedNodeAddress"))

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

func connectToP2P(ip string) {
	controldBase := config.GetString("ControldProtocol") + "://" + config.GetString("ControldHostname") + ":" + config.GetString("ControldPort") + "/api/p2p"
	// Join the p2p network and handle any failures
	joinString := []byte(`{"ip":"` + ip + `"}`)
	resp, err := http.Post(controldBase+"/network/join", "application/json", bytes.NewBuffer(joinString))
	if err != nil {
		fmt.Println(err)
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}
	body, _ := ioutil.ReadAll(resp.Body)
	success, err := jsonparser.GetBoolean(body, "success")
	if !success || err != nil {
		fmt.Println(controldBase + "/network/join")
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}

	// Tell the network our IP and handle any failures
	myIP, err := ipify.GetIp()
	if err != nil {
		fmt.Println("Couldn't get my IP address:", err)
	}
	ipString := []byte(`{"message": {"node": {"ip_address": "` + myIP + `"}}}`)
	resp, err = http.Post(controldBase+"/message/sign", "application/json", bytes.NewBuffer(ipString))
	if err != nil {
		fmt.Println("error signing")
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}
	body, _ = ioutil.ReadAll(resp.Body)
	success, err = jsonparser.GetBoolean(body, "success")
	if !success || err != nil {
		fmt.Println("error signing2")
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}

	// Get the signed message
	signedMessageBytes, _, _, err := jsonparser.Get(body, "response")
	if err != nil {
		return
	}

	// Send the signed message to the p2p network introducing ourselves
	resp, err = http.Post(controldBase+"/state/push_message", "application/json", bytes.NewBuffer(signedMessageBytes))
	if err != nil {
		fmt.Println("error pushing")
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}

	body, _ = ioutil.ReadAll(resp.Body)
	success, err = jsonparser.GetBoolean(body, "success")
	if !success || err != nil {
		fmt.Println("error pushing2")
		time.Sleep(1 * time.Second)
		go connectToP2P(ip)
		return
	}
}
