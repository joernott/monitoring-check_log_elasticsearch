package elasticsearch

import (
	"encoding/json"
	"fmt"
	"strings"

	//"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog/log"
)

type ElasticsearchResult struct {
	PitId        string                       `json:"pit_id"`
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
	Index  string        `json:"_index"`
	Type   string        `json:"_type"`
	Id     string        `json:"_id"`
	Score  float64       `json:"_score"`
	Source HitElement    `json:"_source"`
	Fields HitElement    `json:"fields"`
	Sort   []interface{} `json:"sort"`
}

type ElasticsearchHitTotal struct {
	Value    int64  `json:"total"`
	Relation string `json:"relation"`
}

type AggregationResult map[string]interface{}

type HitElement map[string]interface{}

func (e *Elasticsearch) Search(Index string, Query string) (*ElasticsearchResult, error) {
	var ResultJson *ElasticsearchResult

	logger := log.With().Str("func", "Search").Str("package", "elasticsearch").Logger()
	ResultJson = new(ElasticsearchResult)
	endpoint := "/_search"
	if len(Index) > 0 {
		endpoint = "/" + Index + "/_search"
	}

	logger.Debug().Str("id", "DBG10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Execute Query")
	result, err := e.Connection.Post(endpoint, []byte(Query))
	if result != nil {
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
	}
	if err != nil {
		logger.Error().Str("id", "ERR10020002").Err(err).Msg("Query failed")
		return ResultJson, err
	}
	logger.Info().Str("id", "INF10020001").Str("query", Query).Str("endpoint", endpoint).Msg("Successfully executed query")
	return ResultJson, nil
}

func (haystack HitElement) Get(Needle string) (interface{}, bool) {
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
	return value, true
}

func (haystack HitElement) GetString(Needle string) (string, bool) {
	s, ok := haystack.Get(Needle)
	return fmt.Sprintf("%v", s), ok
}
