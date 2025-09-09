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
	Bool *BoolClause `json: "bool,omitempty"`
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
	if req.StartTime.isValid() && req.EndTime.IsValid() {
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
	return &pb.ProcessHistoryResponse{}, nil
}

func (s *NoxAPIServer) GetTopEvents(ctx context.Context, req *pb.TopNRequest) (*pb.TopNResponse, error) {
	return &pb.TopNResponse{}, nil
}
