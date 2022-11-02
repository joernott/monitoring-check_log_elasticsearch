# monitoring-check_log_elasticsearch

This is a simple Icinga/Nagios check reading log files from elasticsearch

## Usage

As this check uses viper and cobra for commandline, environment and configuration parsing, all commandline flags can also be provided by configuration file, which must then be specified with the *config* parameter or via environment variable. The variables use the prefix "CLE_" and are upper case letters. So, instead of providing the password at the command line (which is then visible in the process list), you can provide it by setting the environment variable "CLE_PASSWORD" or putting it into the config file. Choose your poison.

### Without command

Calling check_log_elasticsearch without any command verb will output the help page. You need to provide one of the available commands to get soemthing useful done.

```bash
check_log_elasticsearch checks log files stored in an elasticsearch cluster and allows for complex filters and multiple ways of colle
cting statistics.

Usage:
  check_log_elasticsearch [flags]
  check_log_elasticsearch [command]

Available Commands:
  check       Check logs
  completion  Generate the autocompletion script for the specified shell
  handle      Handle a history entry
  help        Help about any command
  list        List history entries
  rm          Remove a history entry

Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -h, --help                help for check_log_elasticsearch
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -C, --showcommand         Show the commands for handle etc.

Use "check_log_elasticsearch [command] --help" for more information about a command.
```

### Run check

```bash
Usage:
  check_log_elasticsearch check [flags]

Flags:
  -h, --help              help for check
  -H, --host string       Hostname of the server (default "localhost")
  -p, --password string   Password for the Elasticsearch user (consider using the env variable CLE_PASSWORD instead of passing it via
 commandline)
  -P, --port int          Network port (default 9200)
  -y, --proxy string      Proxy (defaults to none)
  -Y, --socks             This is a SOCKS proxy
  -s, --ssl               Use SSL (default true)
  -T, --timeout string    Timeout understood by time.ParseDuration (default "2m")
  -u, --user string       Username for Elasticsearch
  -v, --validatessl       Validate SSL certificate (default true)

Global Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are speci
fied)
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -C, --showcommand         Show the commands for handle etc.
```

The connection to elasticsearch must be specified by providing the *host*, *port*, *user* and *password* flags. Optionally, SSL can be turned off, by using "--ssl=false" and certificate validation can be turned off with the "--validatessl=false" flag. If you require a proxy, this can be provided by the *proxy* flag, use *socks*, if it is a socks proxy.

The *actionfile* is a configuration file in yaml format which specifies elasticsearch queries and rules to process them. One file can contain multiple actions. By default, all actions are executed
sequentially. If one or more action names are specified with the *action* flag, only those will be run.

By default, only warnings and errors are being logged. The *loglevel* and *logfile* flags control the logging options.

### List check history

When setting the *history* field for an action, check_log_elasticsearch will remember "bad" check runs (Result != OK) for the given amount of seconds. The list of remembered results can be listed with the command *list*. Every entry in this list has a UUID, which can be used to either remove it prematurely or declare the entry as "handled"

```bash
Usage:
  check_log_elasticsearch list [flags]

Flags:
  -h, --help        help for list
  -i, --highlight   Highlight UUID

Global Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -C, --showcommand         Show the commands for handle etc.
```

### Handle a history entry

You can declare a history entry as "handled"  by calling check_log_elasticsearch providing the actionfile and one or more uuids. If you provide the name of an action, this can be limited to the specific action only.

```bash
Usage:
  check_log_elasticsearch handle [flags]

Flags:
  -A, --all            Clear all entries from history
  -h, --help           help for handle
  -U, --uuid strings   Clear entry with the given uuid from history

Global Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -C, --showcommand         Show the commands for handle etc.
```

### Remove an entry from history

The history is pruned on every check for the respective action when it is older than the number of seconds in the *history* field. If you don't want to wait for that to happen, you can manually remove entries with the rm command and the uuids you want to remove.

```bash
Usage:
  check_log_elasticsearch rm [flags]

Flags:
  -A, --all            Remove all entries from history
  -h, --help           help for rm
  -U, --uuid strings   Remove entry with the given uuid from history

Global Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -C, --showcommand         Show the commands for handle etc.
```

## Action file

The check uses the action file to specify where to search and how to match the elasticsearch results to multiple rules. This example contains one search in the syslog index and then has a rule for severity warning and one for the severities error, critical, alert and emergency. Every rule has an exclude pattern to ignore lines where the message contains "Dies ist ein Test"

The special marker *\_TIMESTAMP\_* in the query will be replaced by the timestamp stored in the status file on the previous run. This mechanism prevents rereading all entries in the index again and again. If not provided, it will default to 1900-01-01. If you have logs predating this default, you need to create a status file manually and enter the date.

```yaml
---
actions:
  - name: 'syslog'
    history: 86400
    index: 'syslog-*'
    query: '{"query":{"bool":{"must":[{"match":{"agent.hostname":"testvm"}}],"filter":[{"range":{"@timestamp":{"gt":"_TIMESTAMP_"}}}]}},,"fields":["@timestamp","syslog_severity","message","agent.hostname"],"sort":[{"@timestamp":{"order":"asc", "format": "strict_date_optional_time_nanos", "numeric_type" : "date_nanos"}},{"_shard_doc": "desc"}],"_source":false,_PAGINATION_}'
    limit: 100
    statusfile: status_syslog.yaml
    rules:
      warning:
        description: 'Almost all syslog warnings'
        metric_name: 'warning'
        pattern:
          - field: 'syslog_severity'
            regex: 'warning'
        exclude:
          - field: 'message'
            regex: 'Dies ist ein Test'
        use_and: false
        warning: '0:120'
        critical: '0:420'
      error:
        description: 'Almost all errors and worse'
        metric_name: 'error'
        pattern:
          - field: 'syslog_severity'
            regex: 'error'
          - field: 'syslog_severity'
            regex: 'critical'
          - field: 'syslog_severity'
            regex: 'alert'
          - field: 'syslog_severity'
            regex: 'emergency'
        exclude:
          - field: 'message'
            regex: 'Dies ist ein Test'
        use_and: false
        warning: '0:12'
        critical: '0:42'
```

Description of the fields for the actions:

- *name* : Name for the given action, this name can be used to execute only specific actions in this file
- *history* : Number of seconds to remember bad check results
- *index* : A name/Pattern of elasticsearch indices to use for your search
- *query* : The query for elasticsearch in json format. There are currently two special placeholders which will be replaced before executing the query. The placeholder \_TIMESTAMP\_ will be substituted by the last timestamp from a previous run or, if no status file exists, 1900-01-01T00:00:00.000Z. The placeholder \_PAGINATION\_ will be uised to add the fields pit and search_after on every retrieved page. To speed up the operation, reduce network bandwidth and memory usage, it is wise to use '"_source":false' and to only retrieve the fields used in your patterns.
- *limit* : We are using paginated searches with a page size of 1000 lines. This limit specifies the maximum number of pages to retrieve in this run. It must be high enough to keep up with your log volume but not too high for the checkcommand to take too long and run into the Icinga2 timeout for either the checkcommand or the check.
- *statusfile* : This is the file where the check stores the timestamp and history
- *rules*: This is a map/hash of rules to check every result of the search against.

Every rule has a name (key for the hash) and the following fields:

- *description* : A description for the reader fo the file, explaining the purpose of the rule (optional).
- *metric_name* : If this optional field is provided, it will be used instad of the name of the rule for submitting metrics to Icinga/Nagios. Metrics names have to follow certain rules, if the name of your rule doesn't match them, use this field.
- *pattern* : An array of patterns to look for in every elasticsearch hit. If the document matches a pattern, the hit will be counted. Theoretically, this is optional, but without any pattern, the rule will never match.
- *exclude* : If a doucument matches the patterns specified in the *pattern* field, the check will look, if the hit in this array of patterns. If it finds a match, the hit will not be counted. Optional
- *use_and* : Optional (defaults to false). Usually, matching one of the patterns is sufficient to trigger a hit (OR). If all patterns must match to define a hit, set this field to true (AND)
- *warning* : A range for the number of hits since the last check to trigger a warning. See [the nagious plugin guidelines](https://nagios-plugins.org/doc/guidelines.html#THRESHOLDFORMAT) for details. Mandatory, Use "0:" to never trigger warnings.
- *critical* : A range for the number of hits since the last check to trigger a critical alert. See [the nagious plugin guidelines](https://nagios-plugins.org/doc/guidelines.html#THRESHOLDFORMAT) for details. Mandatory, Use "0:" to never trigger critical alerts

A pattern consists of two fields:

- *field* : This is the field in the elasticsearch hit. If you limit the returned fields in your query, make sure to include the fields you use in your pattern.
- *regex* : A golang regular expression matching the [golang re2 syntax](https://github.com/google/re2/wiki/Syntax). TZhe value of the field will be matched against this regex
