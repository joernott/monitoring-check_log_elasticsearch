module github.com/joernott/monitoring-check_log_elasticsearch

go 1.16

replace github.com/riton/nagiosplugin/v2 => github.com/joernott/nagiosplugin/v2 v2.0.3

require (
	github.com/google/uuid v1.3.0
	github.com/joernott/nagiosplugin/v2 v2.0.3
	github.com/rs/zerolog v1.27.0
	github.com/spf13/cobra v1.5.0
	github.com/spf13/viper v1.12.0
	golang.org/x/net v0.0.0-20220708220712-1185a9018129
	gopkg.in/natefinch/lumberjack.v2 v2.0.0
	gopkg.in/yaml.v3 v3.0.1
)
