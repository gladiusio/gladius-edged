package config

import (
	"path/filepath"
	"strings"

	"github.com/gladiusio/gladius-common/pkg/utils"
	"github.com/spf13/viper"
)

// SetupConfig sets up viper and adds our config options
func SetupConfig() (string, error) {
	base, err := utils.GetGladiusBase()
	if err != nil {
		return "Error retrieving base directory", err
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
	var message = "Using provided config file and overriding defaults"
	if err != nil {
		message = "Error reading config file, it may not exist or is corrupted. Using defaults."
	}

	// Build our config options
	buildOptions(base)

	return message, err
}

func buildOptions(base string) {
	// Content
	ConfigOption("ContentDirectory", filepath.Join(base, "content"))
	ConfigOption("ContentPort", "8080")

	// P2P options
	ConfigOption("P2PSeedNodeAddress", "165.227.16.209")
	ConfigOption("P2PSeedNodePort", "7947")
	ConfigOption("DisableAutoJoin", false)
	ConfigOption("OverrideIP", "")
	ConfigOption("DisableHeartbeat", false)

	// Network Gateway options
	ConfigOption("NetworkGatewayHostname", "localhost")
	ConfigOption("NetworkGatewayPort", "3001")
	ConfigOption("NetworkGatewayProtocol", "http")

	// Logging
	ConfigOption("LogLevel", "info")

	// Misc.
	ConfigOption("GladiusBase", base) // Convenient option to have, not needed though
}

func ConfigOption(key string, defaultValue interface{}) string {
	viper.SetDefault(key, defaultValue)

	return key
}
