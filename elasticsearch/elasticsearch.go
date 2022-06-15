// package elasticsearch handles the interaction with elasticsearch
package elasticsearch

import (
	"encoding/json"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/lra"
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
	Error        string                       `json:"error"`
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
	Total    int64                  `json:"total"`
	MaxScore float64                `json:"max_score"`
	Hits     []ElasticsearchHitList `json:"hits"`
}

type ElasticsearchHitList struct {
	Index  string                 `json:"_index"`
	Type   string                 `json:"_type"`
	Id     string                 `json:"_id"`
	Score  float64                `json:"_score"`
	Source map[string]interface{} `json:"_source"`
}

type AggregationResult map[string]interface{}

func NewElasticsearch(SSL bool, Host string, Port int, User string, Password string, ValidateSSL bool, Proxy string, Socks bool) (*Elasticsearch, error) {
	var e *Elasticsearch

	logger := log.With().Str("func", "NewElasticsearch").Str("package", "elasticsearch").Logger()
	e = new(Elasticsearch)

	hdr := make(lra.HeaderList)
	hdr["Content-Type"] = "application/json"
	c, err := lra.NewConnection(SSL,
		Host,
		Port,
		User,
		Password,
		"",
		ValidateSSL,
		Proxy,
		Socks,
		hdr)
	if err != nil {
		logger.Error().Err(err)
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

	logger.Debug().Str("query", Query).Str("endpoint", endpoint).Msg("Execute Query")
	result, err := e.Connection.Post("/"+endpoint, []byte(Query))
	if err != nil {
		logger.Error().Err(err)
		return nil, err
	}
	logger.Info().Str("query", Query).Str("endpoint", endpoint).Msg("Successfully executed query")
	err = json.Unmarshal(result, ResultJson)
	if err != nil {
		logger.Error().Err(err)
		return nil, err
	}

	debugOut, err := json.MarshalIndent(ResultJson, "", "  ")
	if err != nil {
		logger.Error().Err(err)
	}
	log.Debug().Str("json", string(debugOut)).Msg("Query result")
	return ResultJson, nil
}
