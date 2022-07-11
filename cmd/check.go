package cmd

import (
	"fmt"

	"github.com/riton/nagiosplugin/v2"

	"github.com/joernott/monitoring-check_log_elasticsearch/check"
	"github.com/joernott/monitoring-check_log_elasticsearch/elasticsearch"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check logs",
	Long:  `Check logs in elasticsearch`,
	PersistentPreRun: func(ccmd *cobra.Command, args []string) {
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
		defer nagios.Finish()
		nagios.AddResult(nagiosplugin.OK, "All systems are functioning within normal parameters")

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
			log.Fatal().Err(err).Msg("UNKNOWN: Could not create connection to Elasticsearch")
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create connection to Elasticsearch")
			return
		}
		c, err = check.NewCheck(viper.GetString("actionfile"), elasticsearch, nagios)
		if err != nil {
			log.Fatal().Err(err).Msg("UNKNOWN: Could not create check")
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not create check")
			return
		}
		err = c.Execute(viper.GetStringSlice("action"))
		if err != nil {
			log.Fatal().Msg("UNKNOWN: Could not execute check")
			nagios.AddResult(nagiosplugin.UNKNOWN, "Could not execute check")
			return
		}
		return
	},
}
