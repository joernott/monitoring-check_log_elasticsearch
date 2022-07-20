package cmd

import (
	"fmt"

	"github.com/joernott/nagiosplugin/v2"

	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/check"
	"github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch/elasticsearch"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The subcommand "check" to execute a check. This is called by Nagios/Icinga2
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check logs",
	Long:  `Check logs in elasticsearch`,
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

		nagios := nagiosplugin.NewCheck()
		nagios.SetVerbosity(nagiosplugin.VERBOSITY_MULTI_LINE)
		defer nagios.Finish()

		parsedTimeout, err := parseTimeout(viper.GetString("timeout"))
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not parse timeout")
			return
		}

		elasticsearch, err := elasticsearch.NewElasticsearch(
			viper.GetBool("ssl"),
			viper.GetString("host"),
			viper.GetInt("port"),
			viper.GetString("user"),
			viper.GetString("password"),
			viper.GetBool("validatessl"),
			viper.GetString("proxy"),
			viper.GetBool("socks"),
			parsedTimeout,
		)
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create connection to Elasticsearch")
			log.Fatal().Err(err).Msg("Could not create connection to Elasticsearch")
			nagios.Finish()
			return
		}
		c, err = check.NewCheck(viper.GetString("actionfile"), elasticsearch, nagios)
		if err != nil {
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create check")
			log.Fatal().Err(err).Msg("Could not create check")
			nagios.Finish()
			return
		}
		err = c.Execute(viper.GetStringSlice("action"))
		if err != nil {
			return
		}
		log.Info().Msg("Check finished successfully")
		nagios.Finish()
		return
	},
}
