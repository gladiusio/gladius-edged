package edged

import (
	"os"
	"os/signal"

	"github.com/gladiusio/gladius-edged/edged/config"
	"github.com/gladiusio/gladius-edged/edged/p2p/handler"
	"github.com/gladiusio/gladius-edged/edged/server/contserver"
	"github.com/gladiusio/gladius-edged/edged/state"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Run - Start a web server
func Run() {
	log.Info("Loading config")

	// Setup config handling
	config.SetupConfig()

	// Setup logging level
	switch loglevel := viper.GetString("LogLevel"); loglevel {
	case "debug":
		log.SetLevel(log.DebugLevel)
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	default:
		log.SetLevel(log.InfoLevel)
	}

	log.Info("Starting content server on port: " + viper.GetString("ContentPort"))

	// Create a p2p handler
	controldBase := viper.GetString("NetworkGatewayProtocol") + "://" + viper.GetString("NetworkGatewayHostname") + ":" + viper.GetString("NetworkGatewayPort") + "/api/p2p"
	// TODO: Get seed node from the blockchain
	p2pHandler := handler.New(controldBase,
		viper.GetString("P2PSeedNodeAddress"),
		viper.GetString("P2PSeedNodePort"),
		viper.GetString("ContentPort"))
	go p2pHandler.Connect()

	// Create new thread safe state of the networkd
	s := state.New(p2pHandler)

	// Create a content server
	cs := contserver.New(s, viper.GetString("ContentPort"))
	cs.Start()
	defer cs.Stop()

	log.Info("Started HTTP server.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	p2pHandler.LeaveIfJoined()
}
