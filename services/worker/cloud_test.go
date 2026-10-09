package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestCredentialNamesFixtureUsesOnlyFixedMarkers(t *testing.T) {
	response, err := controlledEndpoint(context.Background(), events.APIGatewayV2HTTPRequest{
		PathParameters: map[string]string{"mode": "credential-names"},
		Headers:        map[string]string{"key": "caller-value-must-not-be-echoed"},
	})
	if err != nil || response.StatusCode != 200 || !response.IsBase64Encoded {
		t.Fatal("fixture response unavailable")
	}
	raw, err := base64.StdEncoding.DecodeString(response.Body)
	var body map[string]map[string]string
	if err != nil || json.Unmarshal(raw, &body) != nil {
		t.Fatal("fixture encoding invalid")
	}
	for _, name := range []string{"key", "auth", "pwd"} {
		want := "unmapped-fixture-" + name
		if response.Headers[name] != want || body["fields"][name] != want || body["query"][name] != want {
			t.Fatal("fixture did not emit independent fixed markers")
		}
	}
}
