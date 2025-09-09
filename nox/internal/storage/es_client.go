package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"nox/internal/model"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
)

type ESClient struct {
	Client *elasticsearch.Client
}

func NewESClient(address string) (*ESClient, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{address},
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating new client: %s", err)
	}

	return &ESClient{Client: es}, nil
}

func (c *ESClient) IndexEvent(ctx context.Context, event model.Event) {
	jsonData, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal event for Elasticsearch", "error", err, "eventType", event.EventType)
		return
	}

	indexName := strings.ToLower(event.EventType)

	_, err = c.Client.Index(indexName, bytes.NewReader(jsonData))
	if err != nil {
		slog.Error("Failed to index event in Elasticsearch", "error", err)
	}
}

func (c *ESClient) EnsureIndex(ctx context.Context, indexName string) {
	res, err := c.Client.Indices.Exists([]string{indexName})
	if err != nil {
		slog.Error("Failed to check if index exists", "error", err, "index", indexName)
		return
	}

	if res.IsError() && res.StatusCode != 404 {
		slog.Error("Error checking index existence", "status", res.Status(), "index", indexName)
		return
	}

	defer res.Body.Close()

	if res.StatusCode == 200 {
		slog.Debug("Index already exists, skipping creation.", "index", indexName)
		return
	}

	slog.Info("Index not found, creating with mapping.", "index", indexName)
	mapping := `{
		"mappings": {
			"properties": {
				"Timestamp": { "type": "date" },
				"EventType": { "type": "keyword" },
				"Source": 	 { "type": "ip" },
				"Metadata": {
					"properties": {
						"process_name": { "type": "keyword" },
						"command": 		{ "type": "text" },
						"pid":			{ "type": "keyword" },
						"ppid":			{ "type": "keyword" },
						"uid":			{ "type": "keyword" },
						"user":			{ "type": "keyword" },
						"sshd_pid":		{ "type": "keyword" }
					}
				}
			}
		}
	}`

	res, err = c.Client.Indices.Create(indexName, c.Client.Indices.Create.WithBody(strings.NewReader(mapping)))
	if err != nil || res.IsError() {
		slog.Error("Failed to create index", "error", err, "status", res.Status())
	} else {
		slog.Info("Successfully created index with mapping", "index", indexName)
	}

	defer res.Body.Close()
}
