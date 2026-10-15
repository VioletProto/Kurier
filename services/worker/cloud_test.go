package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"
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

func TestWorkflowFixtureBindingAndSafeAudit(t *testing.T) {
	var audit bytes.Buffer
	old := log.Writer()
	log.SetOutput(&audit)
	defer log.SetOutput(old)
	nonce := "927dd093-2ec1-4577-85c9-29e27648f4ee"
	in := events.APIGatewayV2HTTPRequest{PathParameters: map[string]string{"mode": "workflow-token"}, QueryStringParameters: map[string]string{"fixture": nonce}}
	r, e := controlledEndpoint(context.Background(), in)
	raw, decode := base64.StdEncoding.DecodeString(r.Body)
	var issued map[string]string
	if e != nil || decode != nil || json.Unmarshal(raw, &issued) != nil || len(issued["token"]) != 64 {
		t.Fatal("token fixture unavailable")
	}
	in.PathParameters["mode"] = "workflow-consume"
	in.Headers = map[string]string{"Authorization": "Bearer " + issued["token"], "Cookie": "caller-private-cookie"}
	r, e = controlledEndpoint(context.Background(), in)
	raw, decode = base64.StdEncoding.DecodeString(r.Body)
	var reflected map[string]any
	if e != nil || decode != nil || json.Unmarshal(raw, &reflected) != nil || r.StatusCode != 200 || reflected["authorized"] != true || reflected["rawReflection"] != issued["token"] || reflected["renderedReflection"] != "Bearer "+issued["token"] {
		t.Fatal("bounded-header fixture mismatch")
	}
	if strings.Contains(audit.String(), issued["token"]) || strings.Contains(audit.String(), "caller-private-cookie") || strings.Count(audit.String(), nonce) != 2 {
		t.Fatal("fixture audit containment failure")
	}
	in.Headers = nil
	r, _ = controlledEndpoint(context.Background(), in)
	if r.StatusCode != 401 {
		t.Fatal("fixture accepted missing Bearer binding")
	}
	in.QueryStringParameters["fixture"] = "invalid\r\nvalue"
	before := audit.Len()
	r, _ = controlledEndpoint(context.Background(), in)
	if r.StatusCode != 400 || audit.Len() != before {
		t.Fatal("fixture accepted/logged invalid correlation")
	}
}
