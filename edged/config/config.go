package config

import (
	"path/filepath"
	"strings"

	"github.com/gladiusio/gladius-common/pkg/utils"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// SetupConfig sets up viper and adds our config options
func SetupConfig() {
	base, err := utils.GetGladiusBase()
	if err != nil {
		log.Warn().Err(err).Msg("Error retrieving base directory")
	}

	// Add config file name and searching
	viper.SetConfigName("gladius-edged")
	viper.AddConfigPath(base)

	// Setup env variable handling
	viper.SetEnvPrefix("EDGED")
	r := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(r)
	viper.AutomaticEnv()

	// Load config
	err = viper.ReadInConfig()
	if err != nil {
		log.Warn().Err(err).Msg("Error reading config file, it may not exist or is corrupted. Using defaults.")
	}

	// Build our config options
	buildOptions(base)
}

func buildOptions(base string) {
	// Content
	ConfigOption("ContentDirectory", filepath.Join(base, "content"))
	ConfigOption("ContentPort", "8080")

	// P2P options
	ConfigOption("P2PSeedNodeAddress", "165.227.16.209")
	ConfigOption("P2PSeedNodePort", "7947")

	// Network Gateway options
	ConfigOption("ControldHostname", "localhost")
	ConfigOption("ControldPort", "3001")
	ConfigOption("ControldProtocol", "http")

	// Logging
	ConfigOption("LogLevel", "info")

	// Misc.
	ConfigOption("GladiusBase", base) // Convenient option to have, not needed though

}

func ConfigOption(key string, defaultValue interface{}) string {
	viper.SetDefault(key, defaultValue)

	return key
}
