package execution

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/json"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"reflect"
	"slices"
	"strings"
	"time"
)

type MCPPair struct {
	ProjectIDs []string `json:"projectIds"`
}
type MCPPairResponse struct {
	Credential wire.Credential `json:"credential"`
	Token      *string         `json:"token"`
}

func mcpView(r row) wire.Credential {
	return wire.Credential{TokenID: r.MCPTokenID, ProjectIDs: r.MCPProjectIDs, CreatedAt: r.CreatedAt, ExpiresAt: r.ExpiresAt, RevokedAt: r.RevokedAt}
}
func mcpFields(item map[string]dt.AttributeValue, extra string) bool {
	allowed := map[string]bool{}
	for _, k := range strings.Fields("PK SK kind schemaVersion version " + extra) {
		allowed[k] = true
	}
	for k := range item {
		if !allowed[k] {
			return false
		}
	}
	return true
}
func knownMCP(item map[string]dt.AttributeValue, r row) bool {
	if r.SchemaVersion != 1 {
		return false
	}
	encoded, e := attributevalue.MarshalMap(r)
	if e != nil {
		return false
	}
	for k, v := range item {
		if !reflect.DeepEqual(v, encoded[k]) {
			return false
		}
	}
	switch r.Kind {
	case "mcpCredential":
		if !mcpFields(item, "ownerId mcpTokenId mcpProjectIds tokenHash state createdAt expiresAt recoveryGeneration revokedAt DPK DSK") {
			return false
		}
		for i, pid := range r.MCPProjectIDs {
			if !wire.ValidID(pid) || slices.Contains(r.MCPProjectIDs[:i], pid) {
				return false
			}
		}
		return r.PK == "U#"+r.OwnerID && r.SK == "MCPCRED#"+r.MCPTokenID && wire.ValidID(r.MCPTokenID) && len(r.TokenHash) == 64 && len(r.MCPProjectIDs) <= 10 && r.ExpiresAt != "" && r.RecoveryGeneration != "" && (r.State == "active" && len(r.MCPProjectIDs) > 0 || r.State == "revoked")
	case "mcpLookup":
		if !mcpFields(item, "ownerId mcpTokenId tokenHash") {
			return false
		}
		return r.PK == "MCPTOKEN#"+r.TokenHash && len(r.TokenHash) == 64 && r.SK == "META" && wire.ValidID(r.MCPTokenID) && wire.ValidID(r.OwnerID)
	case "mcpSlot":
		if !mcpFields(item, "ownerId mcpTokenId") {
			return false
		}
		return r.PK == "U#"+r.OwnerID && r.SK == "MCP#CURRENT" && wire.ValidID(r.MCPTokenID)
	case "mcpIssuance":
		if !mcpFields(item, "ownerId mcpTokenId inputDigest expiresAt DPK DSK") {
			return false
		}
		return strings.HasPrefix(r.SK, "MCPISSUE#") && len(r.InputDigest) == 64 && r.ExpiresAt != "" && wire.ValidID(r.MCPTokenID)
	case "mcpAudit":
		if !mcpFields(item, "ownerId mcpTokenId mcpTool mcpCode mcpRequestId createdAt expiresAt DPK DSK") {
			return false
		}
		return strings.HasPrefix(r.SK, "AUDIT#") && r.ExpiresAt != "" && wire.ValidID(r.MCPTokenID)
	case "mcpRate":
		if !mcpFields(item, "count expiresAt DPK DSK") {
			return false
		}
		return strings.HasPrefix(r.SK, "MCPRATE#") && r.Count >= 0 && r.ExpiresAt != ""
	}
	return false
}
func (s *Service) mcpStage(ctx context.Context) (row, error) {
	r, e := s.stage(ctx)
	if e != nil || r.MCPReadOnlySchemaVersion != 1 || (r.SavedRequestsSchemaVersion != 7 && r.SavedRequestsSchemaVersion != 8 && r.SavedRequestsSchemaVersion != 9) {
		return row{}, safeError()
	}
	return r, nil
}
func (s *Service) CreateMCP(ctx context.Context, owner, key string, in MCPPair) (MCPPairResponse, bool, error) {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return MCPPairResponse{}, false, e
	}
	user, e := s.userGuard(ctx, owner)
	if e != nil {
		return MCPPairResponse{}, false, e
	}
	if !wire.ValidID(key) || len(s.SigningKey) < 32 || len(in.ProjectIDs) < 1 || len(in.ProjectIDs) > 10 {
		return MCPPairResponse{}, false, failure(400, "invalid_request")
	}
	slices.Sort(in.ProjectIDs)
	actions := []dt.TransactWriteItem{s.stageGuard(stage), user}
	for i, pid := range in.ProjectIDs {
		if !wire.ValidID(pid) || i > 0 && pid == in.ProjectIDs[i-1] {
			return MCPPairResponse{}, false, failure(400, "invalid_request")
		}
		p, e := s.project(ctx, owner, pid)
		if e != nil {
			return MCPPairResponse{}, false, e
		}
		actions = append(actions, s.projectGuard(p, "active"))
	}
	sk := "MCPISSUE#" + s.digest("mcp-issuance-key-v1", []string{s.Stage, owner, key})
	dig := s.digest("mcp-issuance-input-v1", in)
	receipt, e := s.get(ctx, s.Table, "U#"+owner, sk)
	if e == nil {
		if receipt.Kind != "mcpIssuance" || receipt.InputDigest != dig || receipt.ExpiresAt <= stamp(s.now()) {
			return MCPPairResponse{}, false, failure(409, "conflict")
		}
		c, e := s.get(ctx, s.Table, receipt.PK, "MCPCRED#"+receipt.MCPTokenID)
		if e != nil {
			return MCPPairResponse{}, false, e
		}
		actions = append(actions, s.rowGuard(receipt), s.rowGuard(c))
		if e = s.transact(ctx, actions); e != nil {
			return MCPPairResponse{}, false, safeError()
		}
		return MCPPairResponse{Credential: mcpView(c)}, false, nil
	}
	if !missingRow(e) {
		return MCPPairResponse{}, false, e
	}
	if e = s.mcpRate(ctx, owner, "issuance", s.now().UTC().Format("2006-01-02T15"), 5, 1); e != nil {
		return MCPPairResponse{}, false, e
	}
	slot, e := s.get(ctx, s.Table, "U#"+owner, "MCP#CURRENT")
	fresh := missingRow(e)
	if e != nil && !fresh {
		return MCPPairResponse{}, false, e
	}
	if !fresh {
		old, e := s.get(ctx, s.Table, slot.PK, "MCPCRED#"+slot.MCPTokenID)
		if e != nil && !missingRow(e) {
			return MCPPairResponse{}, false, e
		}
		if e == nil && old.State == "active" && old.ExpiresAt > stamp(s.now()) && old.RecoveryGeneration == stage.RecoveryGeneration {
			return MCPPairResponse{}, false, failure(409, "conflict")
		}
	}
	tok := randomToken()
	cid := id()
	c := row{PK: "U#" + owner, SK: "MCPCRED#" + cid, Kind: "mcpCredential", SchemaVersion: 1, OwnerID: owner, MCPTokenID: cid, MCPProjectIDs: in.ProjectIDs, TokenHash: tokenHash(tok), State: "active", CreatedAt: stamp(s.now()), ExpiresAt: stamp(s.now().Add(30 * 24 * time.Hour)), RecoveryGeneration: stage.RecoveryGeneration}
	c.DPK = dueShard(owner)
	c.DSK = stamp(s.now().Add(37*24*time.Hour)) + "#" + cid
	slot = row{PK: c.PK, SK: "MCP#CURRENT", Kind: "mcpSlot", SchemaVersion: 1, OwnerID: owner, MCPTokenID: cid, Version: slot.Version}
	if !fresh {
		slot.Version++
	}
	receipt = row{PK: c.PK, SK: sk, Kind: "mcpIssuance", SchemaVersion: 1, OwnerID: owner, MCPTokenID: cid, InputDigest: dig, ExpiresAt: stamp(s.now().Add(7 * 24 * time.Hour)), DPK: dueShard(owner), DSK: stamp(s.now().Add(7*24*time.Hour)) + "#" + id()}
	lookup := row{PK: "MCPTOKEN#" + c.TokenHash, SK: "META", Kind: "mcpLookup", SchemaVersion: 1, OwnerID: owner, MCPTokenID: cid, TokenHash: c.TokenHash}
	for _, ent := range []struct {
		r     row
		fresh bool
	}{{c, true}, {slot, fresh}, {receipt, true}, {lookup, true}} {
		p, e := s.put(s.Table, ent.r, ent.fresh)
		if e != nil {
			return MCPPairResponse{}, false, e
		}
		actions = append(actions, p)
	}
	for _, pid := range in.ProjectIDs {
		link := row{PK: "P#" + pid, SK: "MCP#" + cid, Kind: "executionMCPScope", SchemaVersion: 1, OwnerID: owner, ProjectID: pid, MCPTokenID: cid}
		p, e := s.put(s.Table, link, true)
		if e != nil {
			return MCPPairResponse{}, false, e
		}
		actions = append(actions, p)
	}
	if e = s.transact(ctx, actions); e != nil {
		return MCPPairResponse{}, false, safeError()
	}
	return MCPPairResponse{Credential: mcpView(c), Token: &tok}, true, nil
}
func (s *Service) CurrentMCP(ctx context.Context, owner string) (*wire.Credential, error) {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return nil, e
	}
	u, e := s.userGuard(ctx, owner)
	if e != nil {
		return nil, e
	}
	slot, e := s.get(ctx, s.Table, "U#"+owner, "MCP#CURRENT")
	if missingRow(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	c, e := s.get(ctx, s.Table, slot.PK, "MCPCRED#"+slot.MCPTokenID)
	if missingRow(e) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), u, s.rowGuard(slot), s.rowGuard(c)}); e != nil {
		return nil, safeError()
	}
	if c.RecoveryGeneration != stage.RecoveryGeneration {
		return nil, nil
	}
	v := mcpView(c)
	return &v, nil
}
func (s *Service) RevokeMCP(ctx context.Context, owner, cid string) error {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return e
	}
	u, e := s.userGuard(ctx, owner)
	if e != nil {
		return e
	}
	c, e := s.get(ctx, s.Table, "U#"+owner, "MCPCRED#"+cid)
	if e != nil {
		return e
	}
	if c.Kind != "mcpCredential" || c.OwnerID != owner {
		return failure(404, "not_found")
	}
	if c.State == "revoked" {
		return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), u, s.rowGuard(c)})
	}
	c.State = "revoked"
	c.RevokedAt = pointer(stamp(s.now()))
	c.DPK = dueShard(owner)
	c.DSK = stamp(s.now().Add(7*24*time.Hour)) + "#" + cid
	c.Version++
	p, e := s.put(s.Table, c, false)
	if e != nil {
		return e
	}
	return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), u, p})
}
func (s *Service) authenticateMCP(ctx context.Context, token string) (row, row, error) {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return row{}, row{}, e
	}
	deny := failure(401, "unauthenticated")
	raw, e := base64.RawURLEncoding.DecodeString(token)
	if e != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != token {
		return row{}, row{}, deny
	}
	clear(raw)
	l, e := s.get(ctx, s.Table, "MCPTOKEN#"+tokenHash(token), "META")
	if e != nil || l.Kind != "mcpLookup" {
		return row{}, row{}, deny
	}
	c, e := s.get(ctx, s.Table, "U#"+l.OwnerID, "MCPCRED#"+l.MCPTokenID)
	if e != nil || c.Kind != "mcpCredential" || !hmac.Equal([]byte(c.TokenHash), []byte(tokenHash(token))) || c.State != "active" || c.RecoveryGeneration != stage.RecoveryGeneration || c.ExpiresAt <= stamp(s.now().Add(5*time.Second)) {
		return row{}, row{}, deny
	}
	if _, e = s.userGuard(ctx, c.OwnerID); e != nil {
		return row{}, row{}, deny
	}
	return stage, c, nil
}

// A fixed bucket is bounded across processes; the short bucket enforces burst10.
func (s *Service) mcpRate(ctx context.Context, owner, kind, window string, limit, amount int64) error {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return e
	}
	sk := "MCPRATE#" + kind + "#" + window
	r, e := s.get(ctx, s.Table, "U#"+owner, sk)
	fresh := missingRow(e)
	if e != nil && !fresh {
		return e
	}
	if !fresh && r.Kind != "mcpRate" {
		return safeError()
	}
	if amount < 0 || r.Count+amount > limit {
		return failure(429, "rate_limited")
	}
	r.PK = "U#" + owner
	r.SK = sk
	r.Kind = "mcpRate"
	r.SchemaVersion = 1
	r.Count += amount
	r.ExpiresAt = stamp(s.now().Add(2 * time.Hour))
	r.DPK = dueShard(owner)
	r.DSK = r.ExpiresAt + "#" + id()
	if !fresh {
		r.Version++
	}
	p, e := s.put(s.Table, r, fresh)
	if e != nil {
		return e
	}
	if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), p}); e != nil {
		return safeError()
	}
	return nil
}
func mcpRaw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

// CheckMCPCredential authorizes the bridge before parsing user-supplied inputs;
// the actual release still checks the credential again in its audit transaction.
func (s *Service) CheckMCPCredential(ctx context.Context, token string) error {
	_, _, e := s.authenticateMCP(ctx, token)
	return e
}
