# monitoring-check_log_elasticsearch

This is a simple Icinga/Nagios check reading log files from elasticsearch

## Usage

```bash
  check_log_elasticsearch [flags]

Flags:
  -a, --action strings      Name(s) of action(s) to run (can be used multiple times, default is all, if no explicit actions are specified)   
  -f, --actionfile string   Action file (default "/etc/icinga2/check_log_elasticsearch/actions.yaml")
  -c, --config string       Configuration file
  -h, --help                help for check_log_elasticsearch
  -H, --host string         Hostname of the server (default "localhost")
  -L, --logfile string      Log file (use - to log to stdout) (default "/var/log/icinga2/check_log_elasticsearch.log")
  -l, --loglevel string     Log level (default "WARN")
  -p, --password string     Password for the Elasticsearch user
  -P, --port int            Network port (default 9200)
  -y, --proxy string        Proxy (defaults to none)
  -Y, --socks               This is a SOCKS proxy
  -s, --ssl                 Use SSL (default true)
  -t, --statusfile string   File to remember the last status (default "/var/cache/icinga2/check_log_elasticsearch/status.txt")
  -u, --user string         Username for Elasticsearch
  -v, --validatessl         Validate SSL certificate (default true)
```

As this check uses viper and cobra for commandline, environment and configuration parsing, all commandline flags can also be provided by configuration file, which must then be specified with the *config* parameter or via environment variable. The variables use the prefix "CLE_" and are upper case letters. So, instead of providing the password at the command line (which is then visible in the process list), you can provide it by setting the environment variable "CLE_PASSWORD" or putting it into the config file. Choose your poison.

The connection to elasticsearch must be specified by providing the *host*, *port*, *user* and *password* flags. Optionally, SSL can be turned off, by using "--nossl" and certificate validation can be turned off with the "--novalidatessl" flag. If you require a proxy, this can be provided by the *proxy* flag, use *socks*, if it is a socks proxy.

The *actionfile* is a configuration file in yaml format which specifies elasticsearch queries and rules to process them. One file can contain multiple actions. By default, all actions are executed
sequentially. If one or more action names are specified with the *action* flag, only those will be run.

By default, only warnings and errors are being logged. The *loglevel* and *logfile* flags control the logging options.

## Action file

The check uses the action file to specify where to search and how to match the elasticsearch results to multiple rules. This example contains one search in the syslog index and then has a rule for severity warning and one for the severities error, critical, alert and emergency. Every rule has an exclude pattern to ignore lines where the message contains "Dies ist ein Test"

The special marker *\_TIMESTAMP\_* in the query will be replaced by the timestamp stored in the status file on the previous run. This mechanism prevents rereading all entries in the index again and again.

```yaml
---
actions:
  - name: 'syslog'
    index: 'syslog-*'
    query: '{"query":{"bool":{"must":[{"match":{"agent.hostname":"testhost1.example.com"}}],"filter":[{"range":{"@timestamp":{"gt":"_TIMESTAMP_"}}}]}},"sort":[{"@timestamp":"asc"}],"size":2000}'
    rules:
      warning:
        description: 'Almost all syslog warnings'
        order: 0
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
        order: 0
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
