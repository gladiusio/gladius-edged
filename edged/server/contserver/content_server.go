package contserver

import (
	"crypto/tls"
	"fmt"
	"net"

	"github.com/gladiusio/gladius-edged/edged/state"
	"github.com/gobuffalo/packr"
	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// ContentServer is a server that serves the gladius content from the state
type ContentServer struct {
	running         bool
	port            string
	contentListener net.Listener
	tlsListener     net.Listener
	state           *state.State
}

// New creates a new content server and starts it
func New(state *state.State, port string) *ContentServer {
	cs := &ContentServer{state: state, running: false, port: port}
	cs.Start()
	return cs
}

// Start starts the content server
func (cs *ContentServer) Start() {
	if !cs.running {
		var err error

		box := packr.NewBox("./keys")
		cert, err := box.Find("cert.pem")
		if err != nil {
			log.Fatal().Err(err).Msg("Error loading certificate")
		}

		privKey, err := box.Find("privkey.pem")
		if err != nil {
			log.Fatal().Err(err).Msg("Error loading private key (tls)")
		}

		// Listen on TLS
		cer, err := tls.X509KeyPair(cert, privKey)
		if err != nil {
			log.Fatal().Err(err).Msg("Error loading certificate")
		}

		config := &tls.Config{Certificates: []tls.Certificate{cer}}
		cs.tlsListener, err = tls.Listen("tcp", ":"+cs.port, config)
		if err != nil {
			log.Fatal().Err(err).Msg("Error starting TLS server on address")
		}

		// Listen for http over tls connection
		tlsServer := fasthttp.Server{Handler: requestHandler(cs.state)}
		go tlsServer.Serve(cs.tlsListener)

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
			fmt.Fprintf(ctx, s.Info())
		case "/version":
			fmt.Fprint(ctx, `{"response":{"version":"0.8.0"}}`)
		default:
			ctx.Error("Unsupported path", fasthttp.StatusNotFound)
		}
	}
}

func contentHandler(ctx *fasthttp.RequestCtx, s *state.State) {
	// URL format like /content?website=REQUESTED_SITE?asset=FILE_HASH
	website := string(ctx.QueryArgs().Peek("website"))
	asset := string(ctx.QueryArgs().Peek("asset"))

	if asset != "" {
		a := s.GetAsset(website, asset)
		if a != nil && len(a) > 0 {
			ctx.SetStatusCode(fasthttp.StatusOK)
			ctx.Write(a)
		} else {
			ctx.SetStatusCode(fasthttp.StatusNotFound)
			ctx.Write([]byte("404 - Asset not found"))
		}
	} else {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.Write([]byte(`Must specify asset in URL, like /content?website=REQUESTED_SITE&asset=FILE_HASH`))
	}
}

func setupCORS(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "authorization")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "HEAD,GET,POST,PUT,DELETE,OPTIONS")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
}
