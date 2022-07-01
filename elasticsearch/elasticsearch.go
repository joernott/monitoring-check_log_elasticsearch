// package elasticsearch handles the interaction with elasticsearch
package elasticsearch

import (
	"encoding/json"
	"fmt"
	"strings"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/lra"
	"github.com/rs/zerolog/log"
)

type Elasticsearch struct {
	Connection *lra.Connection
}

type ElasticsearchResult struct {
	Took         int                          `json:"took"`
	TimedOut     bool                         `json:"timed_out"`
	Shards       ElasticsearchShardResult     `json:"_shards"`
	Hits         ElasticsearchHitResult       `json:"hits"`
	Error        ElasticsearchError           `json:"error"`
	Status       int                          `json:"status"`
	Aggregations map[string]AggregationResult `json:"aggregations"`
}

type ElasticsearchShardResult struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type ElasticsearchHitResult struct {
	Total    ElasticsearchHitTotal  `json:"total"`
	MaxScore float64                `json:"max_score"`
	Hits     []ElasticsearchHitList `json:"hits"`
}

type ElasticsearchHitList struct {
	Index  string     `json:"_index"`
	Type   string     `json:"_type"`
	Id     string     `json:"_id"`
	Score  float64    `json:"_score"`
	Source HitElement `json:"_source"`
	Fields HitElement `json:"fields"`
}

type ElasticsearchHitTotal struct {
	Value    int64  `json:"total"`
	Relation string `json:"relation"`
}

type AggregationResult map[string]interface{}

type HitElement map[string]interface{}

type ElasticsearchError struct {
	RootCause []ElasticsearchErrorRootCause `json:"root_cause"`
	Reason    string                        `json:"reason"`
	Resource  ElasticsearchErrorResource    `json:"resource"`
	IndexUUID string                        `json:"index_uuid"`
	Index     string                        `json:"index"`
}

type ElasticsearchErrorRootCause struct {
	Type      string                     `json:"type"`
	Reason    string                     `json:"reason"`
	Resource  ElasticsearchErrorResource `json:"resource"`
	IndexUUID string                     `json:"index_uuid"`
	Index     string                     `json:"index"`
}

type ElasticsearchErrorResource struct {
	Type string `json:"type"`
	Id   string `json:"id"`
}

func NewElasticsearch(SSL bool, Host string, Port int, User string, Password string, ValidateSSL bool, Proxy string, Socks bool) (*Elasticsearch, error) {
	var e *Elasticsearch

	logger := log.With().Str("func", "NewElasticsearch").Str("package", "elasticsearch").Logger()
	e = new(Elasticsearch)

	hdr := make(lra.HeaderList)
	hdr["Content-Type"] = "application/json"

	logger.Debug().
		Str("id", "DBG10010001").
		Str("host", Host).
		Int("port", Port).
		Str("user", User).
		Str("password", "*").
		Bool("validate_ssl", ValidateSSL).
		Str("proxy", Proxy).Bool("socks", Socks).
		Msg("Create connection")
	c, err := lra.NewConnection(SSL,
		Host,
		Port,
		"",
		User,
		Password,
		ValidateSSL,
		Proxy,
		Socks,
		hdr)
	if err != nil {
		logger.Error().Str("id", "ERR10010001").Err(err).Msg("Failed to create connection")
		return nil, err
	}
	e.Connection = c
	return e, nil
}

func (e *Elasticsearch) Search(Index string, Query string) (*ElasticsearchResult, error) {
	var ResultJson *ElasticsearchResult

	ResultJson = new(ElasticsearchResult)
	logger := log.With().Str("func", "Search").Str("package", "elasticsearch").Logger()
	endpoint := "/" + Index + "/_search"

	logger.Debug().Str("id", "DBG10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Execute Query")
	result, err := e.Connection.Post(endpoint, []byte(Query))
	err2 := json.Unmarshal(result, ResultJson)
	if err2 != nil {
		logger.Error().Str("id", "ERR10020001").Err(err).Msg("Unmarshal failed")
		return nil, err2
	}
	// In case of an emergency, you can enable this. It will essentially dump the data into our log
	/*
		debugOut, err2 := json.MarshalIndent(ResultJson.Hits, "", "  ")
		if err2 != nil {
			logger.Warn().Str("id", "WRN10020001").Err(err2).Msg("Marshal Ident failed")
		} else {
			log.Debug().Str("id", "DBG10020002").Str("json", string(debugOut)).Msg("Query result")
		}
	*/
	if err != nil {
		logger.Error().Str("id", "ERR10020002").Err(err).Msg("Query failed")
		return ResultJson, err
	}
	logger.Info().Str("id", "INF10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Successfully executed query")
	return ResultJson, nil
}

func (haystack HitElement) Get(Needle string) (string, bool) {
	if len(haystack) == 0 {
		return "", false
	}
	n := strings.Split(Needle, ".")
	key := n[0]
	if len(n) > 1 {
		subkeys := strings.Join(n[1:], ".")
		subvalues, ok := haystack[n[0]].(HitElement)
		if !ok {
			return "", ok
		}
		return subvalues.Get(subkeys)
	}
	value, ok := haystack[key]
	if !ok {
		return "", ok
	}
	return fmt.Sprintf("%v", value), true
}
