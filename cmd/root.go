package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/davecgh/go-spew/spew"

	"github.com/joernott/monitoring-check_log_elasticsearch/elasticsearch"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

var rootCmd = &cobra.Command{
	Use:   "gobana",
	Short: "Gobana is a commandline kibana",
	Long:  `A commandline kibana written in go`,
	PersistentPreRun: func(ccmd *cobra.Command, args []string) {
		setupLogging()
		err := HandleConfigFile()
		if err != nil {
			panic(err)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		elasticsearch, err := elasticsearch.NewElasticsearch(
			viper.GetBool("ssl"),
			viper.GetString("host"),
			viper.GetInt("port"),
			viper.GetString("user"),
			viper.GetString("password"),
			viper.GetBool("validatessl"),
			viper.GetString("proxy"),
			viper.GetBool("socks"),
		)
		if err != nil {
			log.Fatal().Msg("UNKNOWN: Could not create connection to Elasticsearch")
			os.Exit(3)
		}
		result, err := elasticsearch.Search("schufa-sys-syslog-*", "{\"query\":{\"match\": {\"agent.hostname\":\"box-krn0-vbx-v01\"}}}")
		if err != nil {
			log.Fatal().Msg("UNKNOWN: Could not run search")
			os.Exit(3)
		}
		spew.Dump(result)
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
var Action string

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	setupLogging()
	//logger := log.With().Str("func", "rootCmd.Run").Str("package", "cmd").Logger()

	rootCmd.PersistentFlags().StringVarP(&ConfigFile, "config", "c", "/etc/icinga2/check_log_elasticsearch.yaml", "Configuration file")
	rootCmd.PersistentFlags().BoolVarP(&UseSSL, "ssl", "s", true, "Use SSL")
	rootCmd.PersistentFlags().BoolVarP(&ValidateSSL, "validatessl", "v", true, "Validate SSL certificate")
	rootCmd.PersistentFlags().StringVarP(&Host, "host", "H", "localhost", "Hostname of the server")
	rootCmd.PersistentFlags().IntVarP(&Port, "port", "P", 9200, "Network port")
	rootCmd.PersistentFlags().StringVarP(&User, "user", "u", "", "Username for Elasticsearch")
	rootCmd.PersistentFlags().StringVarP(&Password, "password", "p", "", "Password for the Elasticsearch user")
	rootCmd.PersistentFlags().StringVarP(&LogLevel, "loglevel", "l", "DEBUG", "Log level")
	rootCmd.PersistentFlags().StringVarP(&LogFile, "logfile", "L", "", "Log file (defaults to stdout)")
	rootCmd.PersistentFlags().StringVarP(&Proxy, "proxy", "y", "", "Proxy (defaults to none)")
	rootCmd.PersistentFlags().BoolVarP(&ProxyIsSocks, "socks", "Y", false, "This is a SOCKS proxy")
	rootCmd.PersistentFlags().StringVarP(&ActionFile, "actionfile", "f", "/etc/icinga2/check_log/elasticsearch/actions.yaml", "Action file")
	rootCmd.PersistentFlags().StringVarP(&Action, "action", "a", "", "Action (default is all, if empty)")

	viper.SetDefault("ssl", false)
	viper.SetDefault("validatessl", true)
	viper.SetDefault("host", "localhost")
	viper.SetDefault("port", 9200)
	viper.SetDefault("user", "")
	viper.SetDefault("password", "")
	viper.SetDefault("loglevel", "DEBUG")
	viper.SetDefault("logfile", "")
	viper.SetDefault("proxy", "")
	viper.SetDefault("socks", false)
	viper.SetDefault("actionfile", "")
	viper.SetDefault("action", "")

	viper.BindPFlag("ssl", rootCmd.PersistentFlags().Lookup("ssl"))
	viper.BindPFlag("validatessl", rootCmd.PersistentFlags().Lookup("validatessl"))
	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("user", rootCmd.PersistentFlags().Lookup("user"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))
	viper.BindPFlag("loglevel", rootCmd.PersistentFlags().Lookup("loglevel"))
	viper.BindPFlag("logfile", rootCmd.PersistentFlags().Lookup("logfile"))
	viper.BindPFlag("proxy", rootCmd.PersistentFlags().Lookup("proxy"))
	viper.BindPFlag("socks", rootCmd.PersistentFlags().Lookup("socks"))
	viper.BindPFlag("actionfile", rootCmd.PersistentFlags().Lookup("actionfile"))
	viper.BindPFlag("action", rootCmd.PersistentFlags().Lookup("action"))

	viper.SetEnvPrefix("cle")
	viper.BindEnv("password")
}

func HandleConfigFile() error {
	logger := log.With().Str("func", "rootCmd.HandleConfigFile").Str("package", "cmd").Logger()
	if ConfigFile != "" {
		logger.Debug().Str("file", ConfigFile).Msg("Read config from " + ConfigFile)
		viper.SetConfigFile(ConfigFile)

		if err := viper.ReadInConfig(); err != nil {
			logger.Error().Err(err)
			return err
		}
	}
	return nil
}

// Configure the logging. Has to be called in the actuall command function
func setupLogging() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	var output io.Writer
	if LogFile == "-" {
		output = os.Stdout
	} else {
		output = &lumberjack.Logger{
			Filename:   LogFile,
			MaxBackups: 10,
			MaxAge:     1,
			Compress:   true,
		}
	}
	log.Logger = zerolog.New(output).With().Timestamp().Logger()

	switch strings.ToUpper(LogLevel) {
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
