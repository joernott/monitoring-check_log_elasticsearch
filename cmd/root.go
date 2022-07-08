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

var rootCmd = &cobra.Command{
	Use:   "check_log_elasticsearch",
	Short: "Check logs stored in elasticsearch",
	Long:  `check_log_elasticsearch checks log files stored in an elasticsearch cluster and allows for complex filters and multiple ways of collecting statistics.`,
	PersistentPreRun: func(ccmd *cobra.Command, args []string) {
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

var ConfigFile string
var UseSSL bool
var ValidateSSL bool
var Host string
var Port int
var User string
var Password string
var LogLevel string
var LogFile string
var Proxy string
var ProxyIsSocks bool
var ActionFile string
var Action []string
var Timeout string
var Uuid []string

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "", "Configuration file")
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "loglevel", "l", "WARN", "Log level")
	rootCmd.PersistentFlags().StringVarP(&LogFile, "logfile", "L", "/var/log/icinga2/check_log_elasticsearch.log", "Log file (use - to log to stdout)")
	rootCmd.PersistentFlags().StringVarP(&ActionFile, "actionfile", "f", "/etc/icinga2/check_log_elasticsearch/actions.yaml", "Action file")
	rootCmd.PersistentFlags().StringSliceVarP(&Action, "action", "a", []string{}, "Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)")

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
	rmCmd.PersistentFlags().StringSliceVarP(&Uuid, "uuid", "U", []string{}, "Clear entry with the given uuid from history")

	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(handleCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(listCmd)

	viper.SetDefault("loglevel", "WARN")
	viper.SetDefault("logfile", "/var/log/icinga2/check_log_elasticsearch.log")
	viper.SetDefault("actionfile", "/etc/icinga2/check_log_elasticsearch/actions.yaml")
	viper.SetDefault("action", "")

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

	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))
	viper.BindPFlag("logfile", rootCmd.PersistentFlags().Lookup("logfile"))
	viper.BindPFlag("actionfile", rootCmd.PersistentFlags().Lookup("actionfile"))
	viper.BindPFlag("action", rootCmd.PersistentFlags().Lookup("action"))

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

	viper.SetEnvPrefix("cle")
	viper.BindEnv("password")
}

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

// Configure the logging. Has to be called in the actuall command function
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

func parseTimeout(timeout string) (time.Duration, error) {
	logger := log.With().Str("func", "rootCmd.parseTimeout").Str("package", "cmd").Logger()
	t, err := time.ParseDuration(timeout)
	if err != nil {
		logger.Error().Err(err).Str("timeout", timeout).Msg("Could not parse timeout")
		return t, err
	}
	return t, nil
}
