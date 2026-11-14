package execution

import (
	"context"
	"encoding/json"
	wire "github.com/VioletProto/Kurier/services/request-runtime/mcpwire"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"slices"
	"strings"
	"time"
)

type MCPScope struct {
	Mode       string   `json:"mode"`
	ProjectIDs []string `json:"projectIds,omitempty"`
}

func (s MCPScope) MarshalJSON() ([]byte, error) {
	if s.Mode == "selected_projects" {
		ids := s.ProjectIDs
		if ids == nil {
			ids = []string{}
		}
		return json.Marshal(struct {
			Mode       string   `json:"mode"`
			ProjectIDs []string `json:"projectIds"`
		}{s.Mode, ids})
	}
	return json.Marshal(struct {
		Mode string `json:"mode"`
	}{s.Mode})
}

// Decode the union before normalization: an automatic grant must not accept
// even an empty/null projectIds field, and selected mode requires the array.
func (s *MCPScope) UnmarshalJSON(raw []byte) error {
	type plain MCPScope
	var v plain
	var fields map[string]json.RawMessage
	if NonNullObject(raw) != nil || StrictJSON(raw, &v) != nil || json.Unmarshal(raw, &fields) != nil {
		return failure(400, "invalid_request")
	}
	if v.Mode == "all_owned_projects" && fields["projectIds"] != nil || v.Mode == "selected_projects" && fields["projectIds"] == nil {
		return failure(400, "invalid_request")
	}
	*s = MCPScope(v)
	if !s.valid() {
		return failure(400, "invalid_request")
	}
	return nil
}
func (s MCPScope) valid() bool {
	if s.Mode == "all_owned_projects" {
		return s.ProjectIDs == nil
	}
	if s.Mode != "selected_projects" || len(s.ProjectIDs) < 1 || len(s.ProjectIDs) > 10 {
		return false
	}
	for i, p := range s.ProjectIDs {
		if !wire.ValidID(p) || slices.Contains(s.ProjectIDs[:i], p) {
			return false
		}
	}
	return true
}

type MCPConnection struct {
	TokenID   string   `json:"tokenId"`
	Version   int64    `json:"version"`
	Scope     MCPScope `json:"scope"`
	CreatedAt string   `json:"createdAt"`
	ExpiresAt string   `json:"expiresAt"`
	RevokedAt *string  `json:"revokedAt"`
	// Explicit compatibility signal: schema-1 credentials cannot be edited.
	Editable bool `json:"editable"`
}
type MCPConnectionPair struct {
	Connection MCPConnection `json:"connection"`
	Token      *string       `json:"token"`
}

func connectionView(c row) MCPConnection {
	mode := c.MCPScopeMode
	if mode == "" {
		mode = "selected_projects"
	}
	scope := MCPScope{Mode: mode, ProjectIDs: c.MCPProjectIDs}
	// A revoked last-project credential has an empty selected set in metadata.
	if mode == "selected_projects" && scope.ProjectIDs == nil {
		scope.ProjectIDs = []string{}
	}
	return MCPConnection{TokenID: c.MCPTokenID, Version: c.Version, Scope: scope, CreatedAt: c.CreatedAt, ExpiresAt: c.ExpiresAt, RevokedAt: c.RevokedAt, Editable: c.SchemaVersion == 2}
}
func (s *Service) CreateMCPConnection(ctx context.Context, owner, key string, scope MCPScope) (MCPConnectionPair, bool, error) {
	c, token, created, e := s.createMCP(ctx, owner, key, scope, false)
	return MCPConnectionPair{Connection: connectionView(c), Token: token}, created, e
}
func (s *Service) CurrentMCPConnection(ctx context.Context, owner string) (*MCPConnection, error) {
	c, e := s.currentMCPRow(ctx, owner)
	if e != nil || c == nil {
		return nil, e
	}
	v := connectionView(*c)
	return &v, nil
}
func (s *Service) UpdateMCPScope(ctx context.Context, owner, cid string, expected int64, scope MCPScope) (MCPConnection, error) {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return MCPConnection{}, e
	}
	if stage.MCPReadOnlySchemaVersion != 2 || !scope.valid() {
		return MCPConnection{}, failure(400, "invalid_request")
	}
	u, e := s.userGuard(ctx, owner)
	if e != nil {
		return MCPConnection{}, e
	}
	c, e := s.get(ctx, s.Table, "U#"+owner, "MCPCRED#"+cid)
	if e != nil {
		return MCPConnection{}, e
	}
	if c.OwnerID != owner || c.Kind != "mcpCredential" {
		return MCPConnection{}, failure(404, "not_found")
	}
	if c.Version != expected {
		return MCPConnection{}, failure(412, "precondition_failed")
	}
	if c.SchemaVersion != 2 || c.State != "active" || c.RecoveryGeneration != stage.RecoveryGeneration || c.ExpiresAt <= stamp(s.now().Add(5*time.Second)) {
		return MCPConnection{}, failure(409, "conflict")
	}
	slices.Sort(scope.ProjectIDs)
	actions := []dt.TransactWriteItem{s.stageGuard(stage), u}
	if c.MCPScopeMode == scope.Mode && slices.Equal(c.MCPProjectIDs, scope.ProjectIDs) {
		actions = append(actions, s.rowGuard(c))
		e = s.transact(ctx, actions)
		return connectionView(c), e
	}
	// Condition every newly selected project's current active ownership.
	for _, pid := range scope.ProjectIDs {
		p, err := s.project(ctx, owner, pid)
		if err != nil {
			return MCPConnection{}, err
		}
		actions = append(actions, s.projectGuard(p, "active"))
		if !slices.Contains(c.MCPProjectIDs, pid) {
			link := row{PK: "P#" + pid, SK: "MCP#" + cid, Kind: "executionMCPScope", SchemaVersion: 1, OwnerID: owner, ProjectID: pid, MCPTokenID: cid}
			put, err := s.put(s.Table, link, true)
			if err != nil {
				return MCPConnection{}, err
			}
			actions = append(actions, put)
		}
	}
	for _, pid := range c.MCPProjectIDs {
		if !slices.Contains(scope.ProjectIDs, pid) {
			link, err := s.get(ctx, s.Table, "P#"+pid, "MCP#"+cid)
			if err != nil && !missingRow(err) {
				return MCPConnection{}, err
			}
			if err == nil {
				actions = append(actions, deleteVersion(s.Table, link))
			}
		}
	}
	c.MCPScopeMode = scope.Mode
	c.MCPProjectIDs = scope.ProjectIDs
	c.Version++
	put, e := s.put(s.Table, c, false)
	if e != nil {
		return MCPConnection{}, e
	}
	actions = append(actions, put)
	audit := row{PK: "U#" + owner, SK: "AUDIT#" + stamp(s.now()) + "#" + id(), Kind: "mcpAudit", SchemaVersion: 1, Version: c.Version, OwnerID: owner, MCPTokenID: cid, MCPTool: "scope_change", MCPCode: scope.Mode, MCPRequestID: id(), CreatedAt: stamp(s.now()), ExpiresAt: stamp(s.now().Add(90 * 24 * time.Hour)), DPK: dueShard(owner)}
	audit.DSK = audit.ExpiresAt + "#" + id()
	put, e = s.put(s.Table, audit, true)
	if e != nil {
		return MCPConnection{}, e
	}
	actions = append(actions, put)
	if e = s.transact(ctx, actions); e != nil {
		if pinConflict(e) {
			return MCPConnection{}, failure(412, "precondition_failed")
		}
		return MCPConnection{}, e
	}
	return connectionView(c), nil
}
func mcpScopeDigest(c row) string {
	// Fixed canonical field order and sorted IDs; token ID/version still bind
	// every cursor independently. No secrets or internal coordinates.
	b, _ := json.Marshal(MCPScope{Mode: c.MCPScopeMode, ProjectIDs: c.MCPProjectIDs})
	return tokenHash(string(b))
}

// Reconstruct only a public-ID continuation into the trusted owner index.
func validProjectPosition(t, id string) bool { return strings.HasSuffix(t, "#"+id) && wire.ValidID(id) }
