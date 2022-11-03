package cmd

import (
	"fmt"
	"os"

	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/check"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The subcommand "init" is called manually to create a status file with initial values
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the status file with the given timestamp",
	Long:  `Initializes the status file for the given action with a current timestamp to prevent the first check from taking too long if
there is a lot of data already in elasticsearch. Usually, the ckec kets killed when taking too long and no status file is written.`,
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
		err = c.InitHistory(viper.GetString("timestamp"))
		if err != nil {
			os.Exit(2)
		}
		return
	},
}
