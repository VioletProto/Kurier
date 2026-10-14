package execution

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	runner "github.com/VioletProto/Kurier/services/request-runtime"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Agent = runner.Agent
type Scope = runner.Scope
type Destination = runner.Destination
type PairInput struct {
	Name   string  `json:"name"`
	Scopes []Scope `json:"scopes"`
}
type PairResponse struct {
	Agent Agent   `json:"agent"`
	Token *string `json:"token"`
}

func tokenHash(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }

// Future/unknown credential fields cannot be silently discarded by a CAS write.
func knownAgent(item map[string]dt.AttributeValue, r row) bool {
	if r.SchemaVersion != 1 {
		return false
	}
	encoded, e := attributevalue.MarshalMap(r)
	if e != nil {
		return false
	}
	for key, value := range item {
		if !reflect.DeepEqual(value, encoded[key]) {
			return false
		}
	}
	switch r.Kind {
	case "localAgentLookup":
		return strings.HasPrefix(r.PK, "AGENTTOKEN#") && len(r.PK) == len("AGENTTOKEN#")+64 && r.SK == "META" && uuidPattern.MatchString(r.OwnerID) && uuidPattern.MatchString(r.LocalAgentID)
	case "localAgentSlot":
		return strings.HasPrefix(r.PK, "U#") && r.SK == "AGENT#CURRENT" && uuidPattern.MatchString(r.LocalAgentID)
	case "localAgentRate":
		return strings.HasPrefix(r.PK, "U#") && strings.HasPrefix(r.SK, "AGENTRATE#") && r.Count >= 0 && r.Count <= 10
	case "localAgentIssuance":
		return strings.HasPrefix(r.PK, "U#") && strings.HasPrefix(r.SK, "AGENTISSUE#") && uuidPattern.MatchString(r.LocalAgentID) && len(r.InputDigest) == 64 && r.ExpiresAt != ""
	case "localAgentCredential":
		if r.PK != "U#"+r.OwnerID || r.SK != "CRED#"+r.LocalAgentID || !uuidPattern.MatchString(r.LocalAgentID) || len(r.TokenHash) != 64 || r.RecoveryGeneration == "" || r.ExpiresAt == "" || len(r.Scopes) > 10 || r.State != "active" && r.State != "revoked" || r.State == "active" && len(r.Scopes) == 0 {
			return false
		}
		for _, scope := range r.Scopes {
			if !uuidPattern.MatchString(scope.ProjectID) || len(scope.Destinations) < 1 || len(scope.Destinations) > 16 {
				return false
			}
			for _, d := range scope.Destinations {
				normalized, e := runner.NormalizeLocalDestination(d)
				if e != nil || normalized != d {
					return false
				}
			}
		}
		return true
	}
	return false
}
func randomToken() string {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		panic("secure randomness unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func agentView(r row) Agent {
	if r.Scopes == nil {
		r.Scopes = []Scope{}
	}
	return Agent{LocalAgentID: r.LocalAgentID, Name: r.Name, Scopes: r.Scopes, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt, LastSeenAt: r.LastSeenAt, RecoveryGeneration: r.RecoveryGeneration}
}
func (s *Service) localStage(ctx context.Context) (row, error) {
	stage, e := s.stage(ctx)
	if e != nil || stage.LocalAgentExecutionsSchemaVersion != 1 {
		return row{}, safeError()
	}
	return stage, nil
}
func (s *Service) rate(ctx context.Context, owner, kind, window string, limit int64) error {
	r, e := s.get(ctx, s.Table, "U#"+owner, "AGENTRATE#"+kind)
	fresh := missingRow(e)
	if e != nil && !fresh {
		return e
	}
	if !fresh && r.Kind != "localAgentRate" {
		return safeError()
	}
	if fresh {
		r = row{PK: "U#" + owner, SK: "AGENTRATE#" + kind, Kind: "localAgentRate", SchemaVersion: 1}
	} else {
		r.Version++
	}
	if r.Month != window {
		r.Month = window
		r.Count = 0
	}
	if r.Count >= limit {
		return failure(429, "rate_limited")
	}
	r.Count++
	a, e := s.put(s.Table, r, fresh)
	if e != nil {
		return e
	}
	stage, e := s.localStage(ctx)
	if e != nil {
		return e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), a}); e != nil {
		return safeError()
	}
	return nil
}
func (s *Service) CreateAgent(ctx context.Context, owner, key string, input PairInput) (PairResponse, bool, error) {
	stage, e := s.localStage(ctx)
	if e != nil {
		return PairResponse{}, false, e
	}
	user, e := s.userGuard(ctx, owner)
	if e != nil {
		return PairResponse{}, false, e
	}
	if !uuidPattern.MatchString(key) || len(s.SigningKey) < 32 {
		return PairResponse{}, false, failure(400, "invalid_request")
	}
	input.Name = strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(input.Name) < 1 || utf8.RuneCountInString(input.Name) > 100 || strings.IndexFunc(input.Name, unicode.IsControl) >= 0 || len(input.Scopes) < 1 || len(input.Scopes) > 10 {
		return PairResponse{}, false, failure(400, "invalid_request")
	}
	guards := []dt.TransactWriteItem{s.stageGuard(stage), user}
	seen := map[string]bool{}
	for i, scope := range input.Scopes {
		if !uuidPattern.MatchString(scope.ProjectID) || seen[scope.ProjectID] {
			return PairResponse{}, false, failure(400, "invalid_request")
		}
		seen[scope.ProjectID] = true
		p, e := s.project(ctx, owner, scope.ProjectID)
		if e != nil {
			return PairResponse{}, false, e
		}
		guards = append(guards, s.projectGuard(p, "active"))
		if len(scope.Destinations) < 1 || len(scope.Destinations) > 16 {
			return PairResponse{}, false, failure(400, "invalid_request")
		}
		ds := map[string]bool{}
		for j, d := range scope.Destinations {
			d, e = runner.NormalizeLocalDestination(d)
			if e != nil {
				return PairResponse{}, false, e
			}
			if ds[d.Origin] {
				return PairResponse{}, false, failure(400, "invalid_request")
			}
			ds[d.Origin] = true
			input.Scopes[i].Destinations[j] = d
		}
	}
	raw, _ := json.Marshal(input.Scopes)
	if len(raw) > 16*1024 {
		return PairResponse{}, false, failure(413, "payload_too_large")
	}
	receiptSK := "AGENTISSUE#" + s.digest("agent-issuance-key-v1", []string{s.Stage, owner, key})
	digest := s.digest("agent-issuance-input-v1", input)
	receipt, e := s.get(ctx, s.Table, "U#"+owner, receiptSK)
	if e == nil {
		if receipt.InputDigest != digest || receipt.ExpiresAt <= stamp(s.now()) {
			return PairResponse{}, false, failure(409, "conflict")
		}
		a, e := s.get(ctx, s.Table, "U#"+owner, "CRED#"+receipt.LocalAgentID)
		if e != nil {
			return PairResponse{}, false, e
		}
		return PairResponse{Agent: agentView(a)}, false, nil
	}
	if !missingRow(e) {
		return PairResponse{}, false, e
	}
	if e = s.rate(ctx, owner, "issuance", s.now().UTC().Format("2006-01-02T15"), 5); e != nil {
		return PairResponse{}, false, e
	}
	slot, e := s.get(ctx, s.Table, "U#"+owner, "AGENT#CURRENT")
	fresh := missingRow(e)
	if e != nil && !fresh {
		return PairResponse{}, false, e
	}
	if !fresh && slot.Kind != "localAgentSlot" {
		return PairResponse{}, false, safeError()
	}
	if slot.LocalAgentID != "" {
		old, e := s.get(ctx, s.Table, slot.PK, "CRED#"+slot.LocalAgentID)
		if e != nil {
			return PairResponse{}, false, e
		}
		if old.State == "active" && old.ExpiresAt > stamp(s.now()) && old.RecoveryGeneration == stage.RecoveryGeneration {
			return PairResponse{}, false, failure(409, "conflict")
		}
	}
	token := randomToken()
	aid := id()
	a := row{PK: "U#" + owner, SK: "CRED#" + aid, Kind: "localAgentCredential", SchemaVersion: 1, OwnerID: owner, LocalAgentID: aid, Name: input.Name, Scopes: input.Scopes, State: "active", CreatedAt: stamp(s.now()), ExpiresAt: stamp(s.now().Add(30 * 24 * time.Hour)), RecoveryGeneration: stage.RecoveryGeneration}
	a.TokenHash = tokenHash(token)
	slot.PK = a.PK
	slot.SK = "AGENT#CURRENT"
	slot.Kind = "localAgentSlot"
	slot.SchemaVersion = 1
	slot.LocalAgentID = aid
	slot.State = "active"
	if !fresh {
		slot.Version++
	}
	receipt = row{PK: a.PK, SK: receiptSK, Kind: "localAgentIssuance", SchemaVersion: 1, InputDigest: digest, LocalAgentID: aid, ExpiresAt: stamp(s.now().Add(7 * 24 * time.Hour))}
	lookup := row{PK: "AGENTTOKEN#" + a.TokenHash, SK: "META", Kind: "localAgentLookup", SchemaVersion: 1, OwnerID: owner, LocalAgentID: aid}
	for _, entry := range []struct {
		r     row
		fresh bool
	}{{a, true}, {slot, fresh}, {receipt, true}, {lookup, true}} {
		put, e := s.put(s.Table, entry.r, entry.fresh)
		if e != nil {
			return PairResponse{}, false, e
		}
		guards = append(guards, put)
	}
	for _, scope := range a.Scopes {
		link := row{PK: "P#" + scope.ProjectID, SK: "AGENT#" + aid, Kind: "executionAgentScope", SchemaVersion: 1, ProjectID: scope.ProjectID, OwnerID: owner, LocalAgentID: aid}
		put, e := s.put(s.Table, link, true)
		if e != nil {
			return PairResponse{}, false, e
		}
		guards = append(guards, put)
	}
	if e = s.transact(ctx, guards); e != nil {
		return PairResponse{}, false, safeError()
	}
	return PairResponse{Agent: agentView(a), Token: &token}, true, nil
}
func (s *Service) CurrentAgent(ctx context.Context, owner string) (*Agent, error) {
	if _, e := s.localStage(ctx); e != nil {
		return nil, e
	}
	if _, e := s.userGuard(ctx, owner); e != nil {
		return nil, e
	}
	slot, e := s.get(ctx, s.Table, "U#"+owner, "AGENT#CURRENT")
	if missingRow(e) || slot.LocalAgentID == "" {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	r, e := s.get(ctx, s.Table, slot.PK, "CRED#"+slot.LocalAgentID)
	if e != nil {
		return nil, e
	}
	v := agentView(r)
	return &v, nil
}
func (s *Service) RevokeAgent(ctx context.Context, owner, aid string) error {
	stage, e := s.localStage(ctx)
	if e != nil {
		return e
	}
	user, e := s.userGuard(ctx, owner)
	if e != nil {
		return e
	}
	a, e := s.get(ctx, s.Table, "U#"+owner, "CRED#"+aid)
	if e != nil && !missingRow(e) {
		return e
	}
	if e != nil || a.Kind != "localAgentCredential" {
		return failure(404, "not_found")
	}
	if a.State == "revoked" {
		return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), user, s.rowGuard(a)})
	}
	slot, e := s.get(ctx, s.Table, a.PK, "AGENT#CURRENT")
	if e != nil {
		return e
	}
	a.State = "revoked"
	a.RevokedAt = pointer(stamp(s.now()))
	a.Version++
	put, e := s.put(s.Table, a, false)
	if e != nil {
		return e
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user, put}
	if slot.LocalAgentID == aid {
		slot.Version++
		slot.State = "revoked"
		put, e = s.put(s.Table, slot, false)
		if e != nil {
			return e
		}
		actions = append(actions, put)
	} // Retain safe current metadata until explicit replacement.
	if e = s.transact(ctx, actions); e != nil {
		return safeError()
	}
	return nil
}
func (s *Service) authenticateAgent(ctx context.Context, token string) (row, row, error) {
	stage, e := s.localStage(ctx)
	if e != nil {
		return row{}, row{}, e
	}
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return row{}, row{}, failure(401, "unauthenticated")
	}
	clear(raw)
	lookup, e := s.get(ctx, s.Table, "AGENTTOKEN#"+tokenHash(token), "META")
	if e != nil || lookup.Kind != "localAgentLookup" {
		return row{}, row{}, failure(401, "unauthenticated")
	}
	a, e := s.get(ctx, s.Table, "U#"+lookup.OwnerID, "CRED#"+lookup.LocalAgentID)
	if e != nil || a.Kind != "localAgentCredential" || a.TokenHash != tokenHash(token) || a.State != "active" || a.RecoveryGeneration != stage.RecoveryGeneration || a.ExpiresAt <= stamp(s.now().Add(5*time.Second)) {
		return row{}, row{}, failure(401, "unauthenticated")
	}
	if _, e = s.userGuard(ctx, a.OwnerID); e != nil {
		return row{}, row{}, failure(401, "unauthenticated")
	}
	return stage, a, nil
}
func (s *Service) AgentSession(ctx context.Context, token string) (Agent, string, error) {
	stage, a, e := s.authenticateAgent(ctx, token)
	if e != nil {
		return Agent{}, "", e
	}
	user, e := s.userGuard(ctx, a.OwnerID)
	if e != nil {
		return Agent{}, "", e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), user, s.rowGuard(a)}); e != nil {
		return Agent{}, "", safeError()
	}
	return agentView(a), stamp(s.now()), nil
}
func agentDestination(a row, project, raw string) (Destination, error) {
	for _, scope := range a.Scopes {
		if scope.ProjectID == project {
			for _, d := range scope.Destinations {
				if _, e := runner.ValidateLocalURL(raw, d); e == nil {
					return d, nil
				}
			}
			return Destination{}, failure(409, "local_permission_required")
		}
	}
	return Destination{}, failure(404, "not_found")
}
func (s *Service) agentByID(ctx context.Context, owner, aid string) (row, error) {
	a, e := s.get(ctx, s.Table, "U#"+owner, "CRED#"+aid)
	stage, se := s.localStage(ctx)
	if e != nil || se != nil || a.Kind != "localAgentCredential" || a.State != "active" || a.ExpiresAt <= stamp(s.now().Add(5*time.Second)) || a.RecoveryGeneration != stage.RecoveryGeneration {
		return row{}, failure(409, "agent_unavailable")
	}
	return a, nil
}
func deleteVersion(table string, r row) dt.TransactWriteItem {
	return dt.TransactWriteItem{Delete: &dt.Delete{TableName: &table, Key: keys(r.PK, r.SK), ConditionExpression: aws.String("#v = :v"), ExpressionAttributeNames: map[string]string{"#v": "version"}, ExpressionAttributeValues: map[string]dt.AttributeValue{":v": num(r.Version)}}}
}

func (s *Service) RevokeAuthenticatedAgent(ctx context.Context, token, aid string) error {
	_, a, e := s.authenticateAgent(ctx, token)
	if e != nil {
		return e
	}
	if a.LocalAgentID != aid {
		return failure(401, "unauthenticated")
	}
	return s.RevokeAgent(ctx, a.OwnerID, aid)
}

func (s *Service) ServerTime() string { return stamp(s.now()) }
func (s *Service) CheckAgentCredential(ctx context.Context, token string) error {
	_, _, e := s.authenticateAgent(ctx, token)
	return e
}
