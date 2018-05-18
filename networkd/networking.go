package networkd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/gladiusio/gladius-networkd/networkd/state"
	"github.com/gladiusio/gladius-networkd/rpc-manager"

	"github.com/gladiusio/gladius-utils/config"
	"github.com/gladiusio/gladius-utils/init/manager"

	"github.com/powerman/rpc-codec/jsonrpc2"
	"github.com/valyala/fasthttp"
)

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

	s := state.New()

	// Get where the content is stored and load into memory
	bundleMap := s.Content()

	// Create some strucs so we can pass info between goroutines
	rpcOut := &rpcmanager.RPCOut{HTTPState: make(chan bool)}
	httpOut := &rpcmanager.HTTPOut{}

	//  -- Content server stuff below --

	// Listen on 8080
	lnContent, err := net.Listen("tcp", ":8080")
	if err != nil {
		panic(err)
	}
	// Create a content server
	server := fasthttp.Server{Handler: requestHandler(httpOut, bundleMap)}
	// Serve the content
	defer lnContent.Close()
	go server.Serve(lnContent)

	// -- RPC Stuff below --

	// Register RPC methods
	rpc.Register(&rpcmanager.GladiusEdge{RPCOut: rpcOut})
	// Setup HTTP handling for RPC on port 5000
	http.Handle("/rpc", jsonrpc2.HTTPHandler(nil))
	lnHTTP, err := net.Listen("tcp", ":5000")
	if err != nil {
		panic(err)
	}
	defer lnHTTP.Close()
	go http.Serve(lnHTTP, nil)

	fmt.Println("Started RPC server and HTTP server.")

	// Forever check through the channels on the main thread
	for {
		select {
		case state := <-(*rpcOut).HTTPState: // If it can be assigned to a variable
			if state {
				newContent, err := net.Listen("tcp", ":8080")
				if err != nil {
					log.Print("Server already running so not starting")
				} else {
					lnContent = newContent
					go server.Serve(lnContent)
					fmt.Println("Started HTTP server (from RPC command)")
				}
			} else {
				lnContent.Close()
				fmt.Println("Stopped HTTP server (from RPC command)")
			}
		}
	}
}

// Return a function like the one fasthttp is expecting
func requestHandler(httpOut *rpcmanager.HTTPOut, bundleMap map[string]map[string]string) func(ctx *fasthttp.RequestCtx) {
	// The actual serving function
	return func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/content":
			setupCORS(ctx)
			contentHandler(ctx, bundleMap)
			// TODO: Write stuff to pass back to httpOut
		case "/status":
			setupCORS(ctx)
			fmt.Fprintf(ctx, "Woah a status")
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}
}

func contentHandler(ctx *fasthttp.RequestCtx, bundleMap map[string]map[string]string) {
	// URL format like /content?website=REQUESTED_SITE?route=test%2Ftest
	website := string(ctx.QueryArgs().Peek("website"))
	route := string(ctx.QueryArgs().Peek("route"))

	ctx.SetStatusCode(fasthttp.StatusOK)
	fmt.Fprintf(ctx, bundleMap[website][route])
}

func setupCORS(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "authorization")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "HEAD,GET,POST,PUT,DELETE,OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
}
