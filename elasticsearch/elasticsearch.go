// package elasticsearch handles the interaction with elasticsearch
package elasticsearch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	//"github.com/davecgh/go-spew/spew"
	"github.com/joernott/monitoring-check_log_elasticsearch/lra"
	"github.com/rs/zerolog/log"
)

type Elasticsearch struct {
	Connection *lra.Connection
}

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

type ElasticsearchErrorResponse struct {
	Error ElasticsearchError `json:"error"`
}
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

type ElasticsearchPitResponse struct {
	Id    string             `json:"id"`
	Error ElasticsearchError `json:"error"`
}

type ElasticsearchPit struct {
	Id        string `json:"id"`
	KeepAlive string `json:"keep_alive"`
}

type ElasticsearchQueryPagination struct {
	Pit         ElasticsearchPit         `json:"pit"`
	SearchAfter ElasticsearchSearchAfter `json:"search_after"`
	Size        uint                     `json:"size"`
}

type ElasticsearchPaginatedSearch struct {
	e           *Elasticsearch
	Index       string
	Query       string
	Pagination  ElasticsearchQueryPagination
	SearchAfter ElasticsearchSearchAfter
	Results     []*ElasticsearchResult
}

type ElasticsearchSearchAfter []interface{}

func NewElasticsearch(SSL bool, Host string, Port int, User string, Password string, ValidateSSL bool, Proxy string, Socks bool, timeout time.Duration) (*Elasticsearch, error) {
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
		hdr,
		timeout)
	if err != nil {
		logger.Error().Str("id", "ERR10010001").Err(err).Msg("Failed to create connection")
		return nil, err
	}
	e.Connection = c
	return e, nil
}

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

func (e *Elasticsearch) Pit(Index string, KeepAlive string) (string, error) {
	var x []byte
	logger := log.With().Str("func", "Pit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchPitResponse)
	endpoint := "/" + Index + "/_pit?keep_alive=" + KeepAlive

	logger.Debug().Str("id", "DBG10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("endpoint", endpoint).Msg("Get Point In Time")

	result, err := e.Connection.Post(endpoint, x)
	err2 := json.Unmarshal(result, ResultJson)
	if err2 != nil {
		logger.Error().Str("id", "ERR10040001").Str("data", string(result[:])).Err(err).Msg("Unmarshal failed")
		return "", err2
	}
	if err != nil {
		logger.Error().Str("id", "ERR10040002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("PIT failed")
		return "", err
	}
	pit := ResultJson.Id
	logger.Info().Str("id", "INF10040001").Str("index", Index).Str("keepalive", KeepAlive).Str("pit", pit).Str("endpoint", endpoint).Msg("Successfully got a pit")
	return pit, nil
}

func (e *Elasticsearch) DeletePit(Pit string) error {
	logger := log.With().Str("func", "DeletePit").Str("package", "elasticsearch").Logger()
	ResultJson := new(ElasticsearchErrorResponse)
	endpoint := "/_pit"
	s := "{\"id\":\"" + Pit + "\"}"

	logger.Debug().Str("id", "DBG10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Delete Point In Time")
	result, err := e.Connection.Delete(endpoint, []byte(s))
	err2 := json.Unmarshal(result, ResultJson)
	if err2 != nil {
		logger.Error().Str("id", "ERR10050001").Err(err).Msg("Unmarshal failed")
		return err2
	}
	if err != nil {
		logger.Error().Str("id", "ERR10050002").Err(err).Str("reason", ResultJson.Error.Reason).Msg("Delete PIT failed")
		return err
	}
	logger.Info().Str("id", "INF10050001").Str("pit", Pit).Str("endpoint", endpoint).Msg("Successfully deleted pit")
	return nil
}

func (e *Elasticsearch) StartPaginatedSearch(Index string, Query string) (*ElasticsearchPaginatedSearch, error) {
	logger := log.With().Str("func", "Search").Str("package", "elasticsearch").Logger()

	Search := new(ElasticsearchPaginatedSearch)
	Search.e = e
	Search.Index = Index
	Search.Query = Query
	Search.Pagination.Size = 1000
	Search.Pagination.Pit.KeepAlive = fmt.Sprintf("%vs", e.Connection.Timeout.Seconds())
	pit, err := e.Pit(Index, Search.Pagination.Pit.KeepAlive)
	if err != nil {
		return nil, err
	}
	Search.Pagination.Pit.Id = pit

	q := strings.Replace(Search.Query, "_PAGINATION_", "\"pit\":{\"id\":\""+pit+"\"},\"size\":1000", -1)
	logger.Debug().Str("id", "DBG10060001").Str("query", q).Int("pagination", len(Search.Results)).Msg("First paginated search")
	result, err := e.Search("", q)
	Search.Results = append(Search.Results, result)
	if err != nil {
		return nil, err
	}
	logger.Debug().Str("id", "DBG10060002").Str("old_pit", Search.Pagination.Pit.Id).Str("new_pit", result.PitId).Int("hits", len(result.Hits.Hits)).Msg("Run of first paginated search complete")
	if len(result.Hits.Hits) > 0 {
		Search.Pagination.SearchAfter = result.Hits.Hits[len(result.Hits.Hits)-1].Sort
	}
	Search.Pagination.Pit.Id = result.PitId
	return Search, nil
}

func (p *ElasticsearchPaginatedSearch) Next() error {
	logger := log.With().Str("func", "ElasticsearchPaginatedSearch.Next").Str("package", "elasticsearch").Logger()

	if len(p.Pagination.SearchAfter) == 0 {
		err := errors.New("Tried to continue after end of search")
		logger.Error().Str("id", "ERR10070001").Err(err).Msg("Failed to cross the border")
		return err
	}
	j, err := json.Marshal(p.Pagination)
	if err != nil {
		logger.Error().Str("id", "ERR10070002").Err(err).Msg("Marshal pagination failed")
		return err
	}
	pagination := string(j[1 : len(j)-1])
	q := strings.Replace(p.Query, "_PAGINATION_", pagination, -1)
	logger.Debug().Str("id", "DBG10070002").Str("query", q).Int("pagination", len(p.Results)).Msg("Paginated search")
	result, err := p.e.Search("", q)
	p.Results = append(p.Results, result)
	if err != nil {
		return err
	}
	logger.Debug().Str("id", "DBG10070003").Str("old_pit", p.Pagination.Pit.Id).Str("new_pit", result.PitId).Int("hits", len(result.Hits.Hits)).Msg("Run of paginated search complete")
	if len(result.Hits.Hits) > 0 {
		p.Pagination.SearchAfter = result.Hits.Hits[len(result.Hits.Hits)-1].Sort
	} else {
		var x []interface{}
		p.Pagination.SearchAfter = x
	}
	p.Pagination.Pit.Id = result.PitId
	return nil
}

func (p *ElasticsearchPaginatedSearch) Close() error {
	return p.e.DeletePit(p.Pagination.Pit.Id)
}
