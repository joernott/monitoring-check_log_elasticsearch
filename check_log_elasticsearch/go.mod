module github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch

go 1.16

replace github.com/riton/nagiosplugin/v2 => github.com/joernott/nagiosplugin/v2 v2.0.3

require (
	github.com/google/uuid v1.3.0
	github.com/joernott/lra v1.0.0-beta2
	github.com/joernott/nagiosplugin/v2 v2.0.3
	github.com/rs/zerolog v1.27.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/viper v1.12.0
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/pelletier/go-toml/v2 v2.0.2 // indirect
	github.com/spf13/afero v1.9.2 // indirect
	github.com/subosito/gotenv v1.4.0 // indirect
	golang.org/x/net v0.0.0-20220809184613-07c6da5e1ced // indirect
	golang.org/x/sys v0.0.0-20220808155132-1c4a2a72c664 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
)
