module github.com/joernott/monitoring-check_log_elasticsearch/check_log_elasticsearch

go 1.16

replace github.com/riton/nagiosplugin/v2 => github.com/joernott/nagiosplugin/v2 v2.0.3

require (
	github.com/google/uuid v1.3.0
	github.com/joernott/lra v1.0.0-beta2
	github.com/joernott/nagiosplugin/v2 v2.0.3
	github.com/pelletier/go-toml/v2 v2.0.2 // indirect
	github.com/rs/zerolog v1.27.0
	github.com/spf13/afero v1.9.0 // indirect
	github.com/spf13/cobra v1.5.0
	github.com/spf13/viper v1.12.0
	github.com/subosito/gotenv v1.4.0 // indirect
	golang.org/x/sys v0.0.0-20220712014510-0a85c31ab51e // indirect
	gopkg.in/ini.v1 v1.66.6 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.1
)
