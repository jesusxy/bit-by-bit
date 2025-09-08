package server

import (
	"bytes"
	"context"
	"encoding/json"
	"nox/internal/model"
	"nox/internal/storage"
	pb "nox/proto"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type MatchClause struct {
	Match map[string]string `json:"match"`
}

type BoolClause struct {
	Must []MatchClause `json:"must"`
}

type Query struct {
	Bool BoolClause `json: "bool"`
}

type ESQuery struct {
	Query Query `json:"query"`
}

type NoxAPIServer struct {
	pb.UnimplementedNoxServiceServer
	esClient *storage.ESClient
}

func NewNoxAPIClient(esClient *storage.ESClient) *NoxAPIServer {
	return &NoxAPIServer{esClient: esClient}
}

func (s *NoxAPIServer) SearchEvents(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error) {
	// 1. Build the Elasticsearch Query using the Go structs
	var mustClauses []MatchClause
	for key, val := range req.Filters {
		mustClauses = append(mustClauses, MatchClause{
			Match: map[string]string{"Metadata." + key: value},
		})
	}

	query := ESQuery{
		Query: Query{
			Bool: BoolClause{
				Must: mustClauses,
			},
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	res, err := s.esClient.Client.Search(
		s.esClient.Client.Search.WithIndex("process_executed"),
		s.esClient.Client.Search.WithBody(bytes.NewReader(queryBytes)),
		s.esClient.Client.Search.WithTrackTotalHits(true),
	)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	// parse JSON
	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return nil, err
	}

	var results []*pb.ProcessExecutionEvent
	if hits, ok := r["hits"].(map[string]interface{})["hits"].([]interface{}); ok {
		for _, hit := range hits {
			source, _ := json.Marshal(hit.(map[string]interface{})["_source"])
			var event model.Event

			if err := json.Unmarshal(source, &event); err != nil {
				continue
			}

			results = append(results, &pb.ProcessExecutionEvent{
				Timestamp:   timestamppb.New(event.Timestamp),
				ProcessName: event.Metadata["process_name"],
				Command:     event.Metadata["command"],
				Pid:         event.Metadata["pid"],
				Ppid:        event.Metadata["ppid"],
				Uid:         event.Metadata["uid"],
			})
		}
	}

	return &pb.SearchResponse{ProcessEvents: results}, nil
}

func (s *NoxAPIServer) GetProcessAncestry(ctx context.Context, req *pb.PIDRequest) (*pb.ProcessHistoryResponse, error) {
	return &pb.ProcessHistoryResponse{}, nil
}

func (s *NoxAPIServer) GetTopEvents(ctx context.Context, req *pb.TopNRequest) (*pb.TopNResponse, error) {
	return &pb.TopNResponse{}, nil
}
