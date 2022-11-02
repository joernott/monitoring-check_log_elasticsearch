package cmd

import (
	"fmt"
	"os"

	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/check"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The subcommand "list" is called manually to list historic entries
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List history entries",
	Long:  `List history entries for a given action or for all of them`,
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
		var command string

		if viper.GetBool("showcommand") {
			command = os.Args[0] + " handle -f "+ viper.GetString("actionfile")
		}
		c, err := check.NewCheck(viper.GetString("actionfile"), nil, nil, command)
		if err != nil {
			log.Fatal().Err(err).Msg("UNKNOWN: Could not create check")
			os.Exit(2)
		}
		err = c.ListHistory(viper.GetStringSlice("action"), viper.GetBool("highlight"))
		if err != nil {
			os.Exit(2)
		}
		return
	},
}
