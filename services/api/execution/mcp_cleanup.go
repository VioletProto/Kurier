package execution

import (
	"context"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"strings"
	"time"
)

func (s *Service) removeMCPScope(ctx context.Context, p, link row) error {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return e
	}
	if p.State != "deleting" || p.OwnerID != link.OwnerID || p.ProjectID != link.ProjectID {
		return safeError()
	}
	c, e := s.get(ctx, s.Table, "U#"+link.OwnerID, "MCPCRED#"+link.MCPTokenID)
	actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "deleting"), deleteVersion(s.Table, link)}
	if missingRow(e) {
		return s.transact(ctx, actions)
	}
	if e != nil {
		return e
	}
	projects := []string{}
	for _, pid := range c.MCPProjectIDs {
		if pid != p.ProjectID {
			projects = append(projects, pid)
		}
	}
	if len(projects) != len(c.MCPProjectIDs) {
		c.MCPProjectIDs = projects
		c.Version++
		if len(projects) == 0 {
			c.State = "revoked"
			c.RevokedAt = pointer(stamp(s.now()))
			c.DSK = stamp(s.now().Add(7*24*time.Hour)) + "#" + c.MCPTokenID
		}
		put, e := s.put(s.Table, c, false)
		if e != nil {
			return e
		}
		actions = append(actions, put)
	} else {
		actions = append(actions, s.rowGuard(c))
	}
	return s.transact(ctx, actions)
}
func (s *Service) cleanMCP(ctx context.Context, r row) error {
	stage, e := s.mcpStage(ctx)
	if e != nil {
		return e
	}
	if r.DSK > stamp(s.now())+"#~" {
		return nil
	}
	if r.Kind != "mcpCredential" {
		if r.ExpiresAt == "" || r.ExpiresAt > stamp(s.now()) {
			return nil
		}
		actions := []dt.TransactWriteItem{s.stageGuard(stage), deleteVersion(s.Table, r)}
		if strings.HasPrefix(r.PK, "P#") {
			p, e := s.project(ctx, r.OwnerID, r.ProjectID)
			if e != nil {
				return e
			}
			actions = append(actions, s.projectGuard(p, "active"))
		}
		return s.transact(ctx, actions)
	}
	// Revoke first. Future reads already deny expiry or the new credential version.
	if r.State == "active" && r.ExpiresAt > stamp(s.now()) {
		return nil
	}
	if r.State == "active" {
		r.Version++
		r.State = "revoked"
		r.RevokedAt = pointer(stamp(s.now()))
		p, e := s.put(s.Table, r, false)
		if e != nil {
			return e
		}
		return s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), p})
	}
	for _, pid := range r.MCPProjectIDs {
		link, e := s.get(ctx, s.Table, "P#"+pid, "MCP#"+r.MCPTokenID)
		if missingRow(e) {
			continue
		}
		if e != nil {
			return e
		}
		p, e := s.get(ctx, s.Table, "P#"+pid, "META")
		if e != nil {
			return e
		}
		if p.State != "active" && p.State != "deleting" {
			return safeError()
		}
		if e = s.transact(ctx, []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, p.State), s.rowGuard(r), deleteVersion(s.Table, link)}); e != nil {
			return e
		}
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), deleteVersion(s.Table, r)}
	lookup, e := s.get(ctx, s.Table, "MCPTOKEN#"+r.TokenHash, "META")
	if e == nil {
		actions = append(actions, deleteVersion(s.Table, lookup))
	} else if !missingRow(e) {
		return e
	}
	slot, e := s.get(ctx, s.Table, r.PK, "MCP#CURRENT")
	if e == nil && slot.MCPTokenID == r.MCPTokenID {
		actions = append(actions, deleteVersion(s.Table, slot))
	} else if e != nil && !missingRow(e) {
		return e
	}
	return s.transact(ctx, actions)
}
