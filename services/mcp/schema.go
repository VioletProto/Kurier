package main

import (
	"errors"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/google/jsonschema-go/jsonschema"
)

type outputEnvelope[T any] struct {
	Data  *T          `json:"data,omitempty"`
	Error *wire.Error `json:"error,omitempty"`
}
type listData[T any] struct {
	SchemaVersion int     `json:"schemaVersion"`
	ServerTime    string  `json:"serverTime"`
	Items         []T     `json:"items"`
	NextCursor    *string `json:"nextCursor"`
}
type executionData struct {
	SchemaVersion int            `json:"schemaVersion"`
	ServerTime    string         `json:"serverTime"`
	Items         []struct{}     `json:"items"`
	NextCursor    *string        `json:"nextCursor"`
	Execution     wire.Execution `json:"execution"`
}
type runData struct {
	SchemaVersion int        `json:"schemaVersion"`
	ServerTime    string     `json:"serverTime"`
	Items         []struct{} `json:"items"`
	NextCursor    *string    `json:"nextCursor"`
	Run           wire.Run   `json:"run"`
}
type evidenceData struct {
	SchemaVersion int           `json:"schemaVersion"`
	ServerTime    string        `json:"serverTime"`
	Items         []struct{}    `json:"items"`
	NextCursor    *string       `json:"nextCursor"`
	Evidence      wire.Evidence `json:"evidence"`
}

func toolOutputSchema(name string) (*jsonschema.Schema, error) {
	var s *jsonschema.Schema
	var e error
	switch name {
	case "list_projects":
		s, e = jsonschema.For[outputEnvelope[listData[wire.Project]]](nil)
	case "list_saved_requests":
		s, e = jsonschema.For[outputEnvelope[listData[wire.Request]]](nil)
	case "list_executions":
		s, e = jsonschema.For[outputEnvelope[listData[wire.Execution]]](nil)
	case "list_workflow_runs":
		s, e = jsonschema.For[outputEnvelope[listData[wire.Run]]](nil)
	case "get_execution":
		s, e = jsonschema.For[outputEnvelope[executionData]](nil)
	case "get_workflow_run":
		s, e = jsonschema.For[outputEnvelope[runData]](nil)
	case "get_execution_evidence":
		s, e = jsonschema.For[outputEnvelope[evidenceData]](nil)
	default:
		return nil, errors.New("unknown tool")
	}
	if e != nil {
		return nil, e
	}
	s.OneOf = []*jsonschema.Schema{{Required: []string{"data"}}, {Required: []string{"error"}}}
	return s, nil
}
