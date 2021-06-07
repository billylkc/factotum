package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/billylkc/factotum/factotum"
	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	url      string
	timeout  int
	verbose  bool
	jsonOnly bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "factotum",
	Short: "A powerful web crawler that does everything.",
	Long:  `A powerful web crawler that does everything.`,
	Example: `
  factotum --url="https://www.mannings.com.hk" --timeout=15 --verbose=false --jsonOnly=true
`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if url == "" {
			return fmt.Errorf("Please input valid url.\n")
		}

		ctx := context.Background()
		err := factotum.Run(ctx, url, timeout, verbose, jsonOnly)
		if err != nil {
			return err
		}

		return nil
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.factotum.yaml)")
	// rootCmd.Flags().BoolP("toggle", "", false, "Help message for toggle")

	rootCmd.Flags().StringVarP(&url, "url", "u", "", "URL of the website")
	rootCmd.Flags().IntVarP(&timeout, "timeout", "t", 15, "Timeout before exit")
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.Flags().BoolVarP(&jsonOnly, "jsonOnly", "j", true, "JSON output only")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigName(".factotum")
	}

	viper.AutomaticEnv() // read in environment variables that match

	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
