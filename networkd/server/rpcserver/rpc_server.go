package rpcserver

import (
	"net"
	"net/http"
	"net/rpc"

	"github.com/gladiusio/gladius-networkd/networkd/state"
	"github.com/powerman/rpc-codec/jsonrpc2"
)

// RPCServer is a server that runs an RPC control server
type RPCServer struct {
	running     bool
	rpcListener net.Listener
	state       *state.State
}

// New creates a new rpc server and starts it
func New(state *state.State) *RPCServer {
	rs := &RPCServer{state: state, running: false}
	rs.Start()
	return rs
}

// Start starts the rpc server
func (rs *RPCServer) Start() {
	if !rs.running {
		var err error
		rs.rpcListener, err = net.Listen("tcp", ":5000")
		if err != nil {
			panic(err)
		}
		// Register RPC methods
		rpc.Register(&GladiusEdge{State: rs.state})
		// Setup HTTP handling for RPC on port 5000
		http.Handle("/rpc", jsonrpc2.HTTPHandler(nil))

		go http.Serve(rs.rpcListener, nil)
		rs.running = true
	}
}

// Stop stops the content server
func (rs *RPCServer) Stop() {
	if rs.running {
		if rs.rpcListener != nil {
			rs.rpcListener.Close()
			rs.running = false
		}
	}
}

// GladiusEdge - Entry for the RPC interface. Methods take the form GladiusEdge.Method
type GladiusEdge struct {
	State *state.State
}

// Start - Start the gladius edge node
func (g *GladiusEdge) Start(vals [2]int, res *string) error {
	g.State.SetContentRunState(true)
	*res = "Started the server"
	return nil
}

// Stop - Stop the gladius edge node
func (g *GladiusEdge) Stop(vals [2]int, res *string) error {
	g.State.SetContentRunState(false)
	*res = "Stopped the server"
	return nil
}

// Status - Get the current status of the network node
func (g *GladiusEdge) Status(vals [2]int, res *string) error {
	if g.State.ShouldBeRunning() {
		*res = "Server is running"
	} else {
		*res = "Server is not running"
	}
	return nil
}
