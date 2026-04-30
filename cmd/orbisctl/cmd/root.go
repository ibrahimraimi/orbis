package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "orbisctl",
	Short: "A CLI for the Orbis Service Registry",
	Long: `orbisctl is the native developer tool for interacting with the Orbis Service Registry.
It allows you to view services, trigger health checks, and manage your cluster directly from the terminal.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.orbis.yaml)")
	rootCmd.PersistentFlags().StringP("url", "u", "http://localhost:8500", "URL of the Consul API")
	viper.BindPFlag("url", rootCmd.PersistentFlags().Lookup("url"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".orbis")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Used config file
	}
}

func getAPIURL() string {
	url := viper.GetString("url")
	return url
}
