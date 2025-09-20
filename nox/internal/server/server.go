package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"nox/internal/model"
	"nox/internal/storage"
	pb "nox/proto"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type ESQuery struct {
	Query *Query              `json:"query,omitempty"`
	Aggs  map[string]any      `json:"aggs,omitempty"`
	Size  *int                `json:"size,omitempty"`
	From  *int                `json:"from,omitempty"`
	Sort  []map[string]string `json:"sort,omitempty"`
}

type MatchClause struct {
	Match map[string]string `json:"match"`
}

type Query struct {
	Bool *BoolClause `json:"bool,omitempty"`
}

type BoolClause struct {
	Must   []any `json:"must"`
	Filter []any `json:"filter,omitempty"`
}

type RangeClause struct {
	Range map[string]TimeRange `json:"range"`
}

type TimeRange struct {
	GTE string `json:"gte"`
	LTE string `json:"lte"`
}

// -----------------------------------------------------------------------------
// NoxAPIServer Implementation
// -----------------------------------------------------------------------------

type NoxAPIServer struct {
	pb.UnimplementedNoxServiceServer
	esClient *storage.ESClient
}

type esSearchResponse struct {
	Hits struct {
		Hits []struct {
			Source model.Event `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type esAggregationResponse struct {
	Aggregations struct {
		TopEvents struct {
			Buckets []struct {
				Key      string  `json:"key"`
				DocCount float64 `json:"doc_count"`
			} `json:"buckets"`
		} `json:"top_events"`
	} `json:"aggregations"`
}

func NewNoxAPIServer(esClient *storage.ESClient) *NoxAPIServer {
	return &NoxAPIServer{esClient: esClient}
}

func (s *NoxAPIServer) SearchEvents(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	slog.Info("Handling SearchEvents request", "filters", req.Filters)

	// 1. Build the Elasticsearch Query using the Go structs
	var mustClauses []any
	for key, val := range req.Filters {
		mustClauses = append(mustClauses, MatchClause{
			Match: map[string]string{"Metadata." + key: val},
		})
	}

	var filterClauses []any
	if req.StartTime.GetSeconds() > 0 && req.EndTime.GetSeconds() > 0 {
		filterClauses = append(filterClauses, RangeClause{
			Range: map[string]TimeRange{
				"Timestamp": {
					GTE: req.StartTime.AsTime().Format(time.RFC3339),
					LTE: req.EndTime.AsTime().Format(time.RFC3339),
				},
			},
		})
	}

	query := ESQuery{
		Query: &Query{
			Bool: &BoolClause{
				Must:   mustClauses,
				Filter: filterClauses,
			},
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		slog.Error("Failed to unmarshal Elasticsearch query", "error", err)
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	slog.Debug("Executing SearchEvents Elasticsearch query", "query", string(queryBytes))

	res, err := s.esClient.Client.Search(
		s.esClient.Client.Search.WithContext(ctx),
		s.esClient.Client.Search.WithIndex("process_executed"),
		s.esClient.Client.Search.WithBody(bytes.NewReader(queryBytes)),
		s.esClient.Client.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		slog.Error("Elasticsearch search request failed", "error", err)
		return nil, fmt.Errorf("search returned an error: %s", err)
	}

	defer res.Body.Close()

	if res.IsError() {
		slog.Error("Elasticsearch search returned an error", "status", res.Status())
		return nil, fmt.Errorf("search returned an error: %s", res.Status())
	}

	// parse JSON
	var r esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		slog.Error("Failed to decode Elasticsearch response", "error", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var results []*pb.ProcessExecutionEvent
	for _, hit := range r.Hits.Hits {
		event := hit.Source

		results = append(results, &pb.ProcessExecutionEvent{
			Timestamp:   timestamppb.New(event.Timestamp),
			ProcessName: event.Metadata["process_name"],
			Command:     event.Metadata["command"],
			Pid:         event.Metadata["pid"],
			Ppid:        event.Metadata["ppid"],
			Uid:         event.Metadata["uid"],
		})
	}

	slog.Info("SearchEvents request completed successfully", "hits", len(results))
	return &pb.SearchResponse{ProcessEvents: results}, nil
}

func (s *NoxAPIServer) GetProcessAncestry(ctx context.Context, req *pb.PIDRequest) (*pb.ProcessHistoryResponse, error) {
	slog.Info("Handling GetProcessAncestry request", "pid", req.Pid)

	var ancestry []*pb.ProcessExecutionEvent
	currentPid := req.Pid
	const maxDepth = 20

	for i := 0; i < maxDepth; i++ {
		if currentPid == "" || currentPid == "0" || currentPid == "1" {
			break
		}

		size := 1
		query := ESQuery{
			Query: &Query{
				Bool: &BoolClause{
					Must: []any{
						MatchClause{
							Match: map[string]string{"Metadata.pid": currentPid},
						},
					},
				},
			},
			Sort: []map[string]string{{"Timestamp": "desc"}},
			Size: &size,
		}

		queryBytes, err := json.Marshal(query)
		if err != nil {
			return nil, fmt.Errorf("failed to build query: %w", err)
		}

		res, err := s.esClient.Client.Search(
			s.esClient.Client.Search.WithContext(ctx),
			s.esClient.Client.Search.WithIndex("process_executed"),
			s.esClient.Client.Search.WithBody(bytes.NewReader(queryBytes)),
		)

		if err != nil || res.IsError() {
			break
		}
		defer res.Body.Close()

		var r esSearchResponse
		if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
			break
		}

		if len(r.Hits.Hits) == 0 {
			break
		}

		event := r.Hits.Hits[0].Source
		ancestry = append(ancestry, &pb.ProcessExecutionEvent{
			Timestamp:   timestamppb.New(event.Timestamp),
			ProcessName: event.Metadata["process_name"],
			Command:     event.Metadata["command"],
			Pid:         event.Metadata["pid"],
			Ppid:        event.Metadata["ppid"],
			Uid:         event.Metadata["uid"],
		})

		currentPid = event.Metadata["ppid"]
	}

	return &pb.ProcessHistoryResponse{Events: ancestry}, nil
}

func (s *NoxAPIServer) GetTopEvents(ctx context.Context, req *pb.TopNRequest) (*pb.TopNResponse, error) {
	slog.Info("Handling GetTopEvents request", "field", req.Field, "n", req.N)
	if req.Field == "" || req.N <= 0 {
		return nil, fmt.Errorf("field must be specified and N must be positive")
	}

	size := 0
	aggregationField := "Metadata." + req.Field

	query := ESQuery{
		Size: &size,
		Aggs: map[string]any{
			"top_events": map[string]any{
				"terms": map[string]any{
					"field": aggregationField,
					"size":  req.N,
				},
			},
		},
	}

	if req.StartTime.GetSeconds() > 0 && req.EndTime.GetSeconds() > 0 {
		query.Query = &Query{
			Bool: &BoolClause{
				Filter: []any{
					RangeClause{
						Range: map[string]TimeRange{
							"Timestamp": {
								GTE: req.StartTime.AsTime().Format(time.RFC3339),
								LTE: req.EndTime.AsTime().Format(time.RFC3339),
							},
						},
					},
				},
			},
		}
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	res, err := s.esClient.Client.Search(
		s.esClient.Client.Search.WithContext(ctx),
		s.esClient.Client.Search.WithIndex("process_executed"),
		s.esClient.Client.Search.WithBody(bytes.NewReader(queryBytes)),
	)

	if err != nil || res.IsError() {
		return nil, fmt.Errorf("aggregation request failed")
	}

	defer res.Body.Close()

	var r esAggregationResponse
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to decode aggregation response: %w", err)
	}

	var results []*pb.TopNResponse_Count
	for _, bucket := range r.Aggregations.TopEvents.Buckets {
		results = append(results, &pb.TopNResponse_Count{
			Item:  bucket.Key,
			Count: int64(bucket.DocCount),
		})
	}

	return &pb.TopNResponse{Results: results}, nil
}
