package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

func (c *ESClient) IndexEvent(ctx context.Context, event model.Event) error {
	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("[es] failed to marshal event for ES: %w - EventType: %s", err, event.EventType)
	}

	indexName := strings.ToLower(event.EventType)

	res, err := c.Client.Index(
		indexName,
		bytes.NewReader(jsonData),
		c.Client.Index.WithContext(ctx),
	)

	if err != nil {
		return fmt.Errorf("[es] failed to index event for ES: %w - IndexName: %s", err, indexName)
	}

	defer res.Body.Close()

	if res.IsError() {
		// Read the full response body to get the detailed error from Elasticsearch.
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("[es] error during indexing. status: %s - indexName: %s - response: %s",
			res.Status(),
			indexName,
			string(body),
		)
	}

	return nil
}

func (c *ESClient) EnsureIndex(ctx context.Context, indexName string) error {
	res, err := c.Client.Indices.Exists([]string{indexName}, c.Client.Indices.Exists.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("[es] failed to check if index exists - error: %w, index: %s", err, indexName)
	}
	defer res.Body.Close()

	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("[es] error checking index existence - status: %s, index: %s", res.Status(), indexName)
	}

	if res.StatusCode == 200 {
		return nil
	}

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

	res, err = c.Client.Indices.Create(
		indexName,
		c.Client.Indices.Create.WithBody(strings.NewReader(mapping)),
		c.Client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("[es] failed to create index - err: %w ", err)
	}

	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("[es] error during index creation. status: %s - indexName: %s - response: %s",
			res.Status(),
			indexName,
			string(body),
		)
	}

	return nil
}
