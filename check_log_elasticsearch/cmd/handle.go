package cmd

import (
	"fmt"
	"os"

	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/check"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The subcommand "handle" is called manually to set historic entries to
// "handled"
var handleCmd = &cobra.Command{
	Use:   "handle",
	Short: "Handle a history entry",
	Long:  `Mark a history entry with the provided uuid as handled`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
		err := HandleConfigFile()
		if err != nil {
			fmt.Println("Config error")
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		var c *check.Check

		c, err := check.NewCheck(viper.GetString("actionfile"), nil, nil, "")
		if err != nil {
			log.Fatal().Err(err).Msg("UNKNOWN: Could not create check")
			os.Exit(2)
		}
		err = c.HandleHistory(viper.GetStringSlice("action"), viper.GetStringSlice("uuid"), viper.GetBool("all"))
		if err != nil {
			os.Exit(2)
		}
		return
	},
}
