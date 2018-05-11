package networkd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"path"
	"strings"

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

	// Get where the content is stored and load into memory
	bundleMap := loadContentFromDisk()

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

func getContentDir() (string, error) {
	// TODO: Actually get correct filepath
	// TODO: Add configurable values from a config file
	contentDir := config.GetString("ContentDirectory")
	if contentDir == "" {
		return contentDir, errors.New("No content directory specified")
	}
	return contentDir, nil
}

// Return a map of the json bundles on disk
func loadContentFromDisk() map[string]map[string]string {
	filePath, err := getContentDir()
	if err != nil {
		panic(err)
	}

	files, err := ioutil.ReadDir(filePath)
	if err != nil {
		log.Fatal("Error when reading content dir: ", err)
	}

	m := make(map[string]map[string]string)

	for _, f := range files {
		website := f.Name()
		if f.IsDir() {
			contentFiles, err := ioutil.ReadDir(path.Join(filePath, website))
			if err != nil {
				log.Fatal("Error when reading content dir: ", err)
			}
			fmt.Println("Loading website: " + website)
			m[website] = make(map[string]string)
			for _, contentFile := range contentFiles {
				// Replace "%2f" with "/" and ".json" with ""
				replacer := strings.NewReplacer("%2f", "/", "%2F", "/", ".html", "")
				contentName := contentFile.Name()

				// Create a route name for the mapping
				routeName := replacer.Replace(contentName)

				// Pull the file
				b, err := ioutil.ReadFile(path.Join(filePath, website, contentName))
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("Loaded route: " + routeName)
				m[website][routeName] = string(b)
			}
		}
	}

	return m
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
