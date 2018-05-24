package contserver

import (
	"fmt"
	"net"

	"github.com/gladiusio/gladius-networkd/networkd/state"
	"github.com/valyala/fasthttp"
)

// ContentServer is a server that serves the gladius content from the state
type ContentServer struct {
	running         bool
	contentListener net.Listener
	state           *state.State
}

// New creates a new content server and starts it
func New(state *state.State) *ContentServer {
	cs := &ContentServer{state: state, running: false}
	cs.Start()
	return cs
}

// Start starts the content server
func (cs *ContentServer) Start() {
	if !cs.running {
		var err error
		cs.contentListener, err = net.Listen("tcp", ":8080")
		if err != nil {
			panic(err)
		}
		// Create a content server
		server := fasthttp.Server{Handler: requestHandler(cs.state)}

		// Serve the content
		go server.Serve(cs.contentListener)

		cs.running = true
	}
}

// Stop stops the content server
func (cs *ContentServer) Stop() {
	if cs.running {
		if cs.contentListener != nil {
			cs.contentListener.Close()
			cs.running = false
		}
	}
}

// Return a function like the one fasthttp is expecting
func requestHandler(s *state.State) func(ctx *fasthttp.RequestCtx) {
	// The actual serving function
	return func(ctx *fasthttp.RequestCtx) {
		setupCORS(ctx)
		switch string(ctx.Path()) {
		case "/content":
			contentHandler(ctx, s)
		case "/status":
			fmt.Fprintf(ctx, "Woah a status")
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}
}

func contentHandler(ctx *fasthttp.RequestCtx, s *state.State) {
	// URL format like /content?website=REQUESTED_SITE?route=test%2Ftest
	website := string(ctx.QueryArgs().Peek("website"))
	route := string(ctx.QueryArgs().Peek("route"))
	asset := string(ctx.QueryArgs().Peek("asset"))

	if asset != "" {
		ctx.SetStatusCode(fasthttp.StatusOK)
		fmt.Fprintf(ctx, s.GetAsset(website, asset))
	} else {
		ctx.SetStatusCode(fasthttp.StatusOK)
		fmt.Fprintf(ctx, s.GetPage(website, route))
	}
}

func setupCORS(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "authorization")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "HEAD,GET,POST,PUT,DELETE,OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
}
