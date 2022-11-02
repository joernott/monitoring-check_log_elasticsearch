package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Calling check_log_elasticsearch without a subcommand will just output the
// generic help.
var rootCmd = &cobra.Command{
	Use:   "check_log_elasticsearch",
	Short: "Check logs stored in elasticsearch",
	Long:  `check_log_elasticsearch checks log files stored in an elasticsearch cluster and allows for complex filters and multiple ways of collecting statistics.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setupLogging()
		err := HandleConfigFile()
		if err != nil {
			fmt.Println("Config error")
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
		return
	},
}

// Global variable for cobra, storing the viper configuration file name
var ConfigFile string

// Global variable for cobra, one of the zerolog log levels (TRACE, DEBUG, INFO,
// WARN, ERROR, FATAL,PANIC). Trace produces an extreme amount of log data, use
// with care on a small dataset
var LogLevel string

// Global variable for cobra, name of the log file, "-" logs to stdout
var LogFile string

// Global variable for cobra, name of the config file describing the actions
var ActionFile string

// Global variable for cobra, list of actions to execute. If empty, all actions
// will be executed
var Action []string

// Global variable for cobra, used in the check subcommand
var UseSSL bool

// Global variable for cobra, validate the SSL certificate (check subcommand)
var ValidateSSL bool

// Global variable for cobra, hostname or IP (check subcommand)
var Host string

// Global variable for cobra, port of Elasticsearch (check subcommand)
var Port int

// Global variable for cobra, User for connecting to  Elasticsearch (check subcommand)
var User string

// Global variable for cobra, Password for connecting to Elasticsearch (check subcommand)
var Password string

//Global variable for cobra, URL of a proxy (check subcommand)
var Proxy string

// Global variable for cobra, is the proxy a Socks proxy (check subcommand)
var ProxyIsSocks bool

// Global variable for cobra, timeout for the checks
var Timeout string

// Global variable for cobra, list of uuids for handle and rm subcommands
var Uuid []string

// Global variable for cobra, handle/rm all uuids
var All bool

// Global variable for cobra, list highlights the UUID
var HighlightUuid bool

// Global variable for cobra, show the rm/handle etc commands
var ShowCommand bool

// Run the checkcommand
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// Initialize the various parameters and set defaults
func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "", "Configuration file")
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "loglevel", "l", "WARN", "Log level")
	rootCmd.PersistentFlags().StringVarP(&LogFile, "logfile", "L", "/var/log/icinga2/check_log_elasticsearch.log", "Log file (use - to log to stdout)")
	rootCmd.PersistentFlags().StringVarP(&ActionFile, "actionfile", "f", "/etc/icinga2/check_log_elasticsearch/actions.yaml", "Action file")
	rootCmd.PersistentFlags().StringSliceVarP(&Action, "action", "a", []string{}, "Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)")
	rootCmd.PersistentFlags().BoolVarP(&ShowCommand, "showcommand", "C", false, "Show the commands for handle etc.")

	checkCmd.PersistentFlags().BoolVarP(&UseSSL, "ssl", "s", true, "Use SSL")
	checkCmd.PersistentFlags().BoolVarP(&ValidateSSL, "validatessl", "v", true, "Validate SSL certificate")
	checkCmd.PersistentFlags().StringVarP(&Host, "host", "H", "localhost", "Hostname of the server")
	checkCmd.PersistentFlags().IntVarP(&Port, "port", "P", 9200, "Network port")
	checkCmd.PersistentFlags().StringVarP(&User, "user", "u", "", "Username for Elasticsearch")
	checkCmd.PersistentFlags().StringVarP(&Password, "password", "p", "", "Password for the Elasticsearch user (consider using the env variable CLE_PASSWORD instead of passing it via commandline)")
	checkCmd.PersistentFlags().StringVarP(&Proxy, "proxy", "y", "", "Proxy (defaults to none)")
	checkCmd.PersistentFlags().BoolVarP(&ProxyIsSocks, "socks", "Y", false, "This is a SOCKS proxy")
	checkCmd.PersistentFlags().StringVarP(&Timeout, "timeout", "T", "2m", "Timeout understood by time.ParseDuration")

	handleCmd.PersistentFlags().StringSliceVarP(&Uuid, "uuid", "U", []string{}, "Clear entry with the given uuid from history")
	rmCmd.PersistentFlags().StringSliceVarP(&Uuid, "uuid", "U", []string{}, "Remove entry with the given uuid from history")
	handleCmd.PersistentFlags().BoolVarP(&All, "all", "A", false, "Clear all entries from history")
	rmCmd.PersistentFlags().BoolVarP(&All, "all", "A", false, "Remove all entries from history")
	listCmd.PersistentFlags().BoolVarP(&HighlightUuid, "highlight", "i", false, "Highlight UUID")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(handleCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(listCmd)

	viper.SetDefault("loglevel", "WARN")
	viper.SetDefault("logfile", "/var/log/icinga2/check_log_elasticsearch.log")
	viper.SetDefault("actionfile", "/etc/icinga2/check_log_elasticsearch/actions.yaml")
	viper.SetDefault("action", "")
	viper.SetDefault("showcommand", false)

	viper.SetDefault("ssl", true)
	viper.SetDefault("validatessl", true)
	viper.SetDefault("host", "localhost")
	viper.SetDefault("port", 9200)
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("proxy", "")
	viper.SetDefault("socks", false)
	viper.SetDefault("timeout", "2m")

	viper.SetDefault("uuid", []string{})
	viper.SetDefault("all", false)
	viper.SetDefault("highlight", false)

	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))
	viper.BindPFlag("logfile", rootCmd.PersistentFlags().Lookup("logfile"))
	viper.BindPFlag("actionfile", rootCmd.PersistentFlags().Lookup("actionfile"))
	viper.BindPFlag("action", rootCmd.PersistentFlags().Lookup("action"))
	viper.BindPFlag("showcommand", rootCmd.PersistentFlags().Lookup("showcommand"))

	viper.BindPFlag("ssl", checkCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("validatessl", checkCmd.PersistentFlags().Lookup("validatessl"))
	viper.BindPFlag("host", checkCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", checkCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("user", checkCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", checkCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("proxy", checkCmd.PersistentFlags().Lookup("proxy"))
	viper.BindPFlag("socks", checkCmd.PersistentFlags().Lookup("socks"))
	viper.BindPFlag("timeout", checkCmd.PersistentFlags().Lookup("timeout"))

	viper.BindPFlag("uuid", handleCmd.PersistentFlags().Lookup("uuid"))
	viper.BindPFlag("uuid", rmCmd.PersistentFlags().Lookup("uuid"))
	viper.BindPFlag("all", handleCmd.PersistentFlags().Lookup("all"))
	viper.BindPFlag("all", rmCmd.PersistentFlags().Lookup("all"))
	viper.BindPFlag("highlight", listCmd.PersistentFlags().Lookup("highlight"))

	viper.SetEnvPrefix("cle")
	viper.BindEnv("password")
}

// Load the configuration file if the parameter is set.
func HandleConfigFile() error {
	logger := log.With().Str("func", "rootCmd.HandleConfigFile").Str("package", "cmd").Logger()
	if ConfigFile != "" {
		logger.Debug().Str("file", ConfigFile).Msg("Read config from " + ConfigFile)
		viper.SetConfigFile(ConfigFile)

		if err := viper.ReadInConfig(); err != nil {
			logger.Error().Err(err).Msg("Could not read config file")
			return err
		}
	}
	return nil
}

// Configure the logging.
func setupLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	var output io.Writer
	logfile := viper.GetString("logfile")
	if logfile == "-" {
		output = os.Stdout
	} else {
		output = &lumberjack.Logger{
			Filename:   logfile,
			MaxBackups: 10,
			MaxAge:     1,
			Compress:   true,
		}
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	switch strings.ToUpper(viper.GetString("loglevel")) {
	case "TRACE":
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	case "DEBUG":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "INFO":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "WARN":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "ERROR":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "FATAL":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "PANIC":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		err := errors.New("Illegal log level " + LogLevel)
		log.Error().Str("id", "ERR00001").Err(err).Msg("")
		os.Exit(3)
	}
	log.Debug().Str("id", "DBG00001").Str("func", "setupLogging").Str("logfile", LogFile).Msg("Logging to " + LogFile)
}

// Parse the timeout string into a go duration
func parseTimeout(timeout string) (time.Duration, error) {
	logger := log.With().Str("func", "rootCmd.parseTimeout").Str("package", "cmd").Logger()
	t, err := time.ParseDuration(timeout)
	if err != nil {
		logger.Error().Err(err).Str("timeout", timeout).Msg("Could not parse timeout")
		return t, err
	}
	return t, nil
}
