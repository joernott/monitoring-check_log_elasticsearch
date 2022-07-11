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
