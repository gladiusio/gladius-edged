package edged

import (
	"os"
	"os/signal"
	"strings"

	"github.com/gladiusio/gladius-edged/edged/config"
	"github.com/gladiusio/gladius-edged/edged/p2p/handler"
	"github.com/gladiusio/gladius-edged/edged/server/contserver"
	"github.com/gladiusio/gladius-edged/edged/state"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// Run - Start a web server
func Run() {
	// Setup config handling
	message, err := config.SetupConfig()

	setupLogger()

	if err != nil {
		log.Warn().Msg(message)
	}

	log.Info().Msg("Starting content server on port: " + viper.GetString("ContentPort"))

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
	cs := contserver.New(s, viper.GetString("ContentPort"), viper.GetString("HTTPPort"))
	cs.Start()
	defer cs.Stop()

	log.Info().Msg("Started HTTPS server.")

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	<-c

	p2pHandler.LeaveIfJoined()
}

func setupLogger() {
	// Setup logging level
	switch loglevel := viper.GetString("Log.Level"); strings.ToLower(loglevel) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warning":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case "disabled":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	if !viper.IsSet("Log.Pretty") || (viper.IsSet("Log.Pretty") && viper.GetBool("Log.Pretty")) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	}
}
