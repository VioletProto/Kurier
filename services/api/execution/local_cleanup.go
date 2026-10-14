package execution

import (
	"context"
	dt "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func (s *Service) clearAgentJob(ctx context.Context, p, j row, retention bool) error {
	a, e := s.get(ctx, s.Table, "U#"+j.OwnerID, "CRED#"+j.LocalAgentID)
	if e != nil {
		return e
	}
	if a.ActiveJob != j.JobID || a.ActiveProject != j.ProjectID {
		return nil
	}
	stage, e := s.localStage(ctx)
	if e != nil {
		return e
	}
	guards, e := s.drainGuards(ctx, stage, p, j.ExecutionID, retention)
	if e != nil {
		return e
	}
	a.ActiveJob = ""
	a.ActiveProject = ""
	a.Version++
	put, e := s.put(s.Table, a, false)
	if e != nil {
		return e
	}
	guards = append(guards, put, s.rowGuard(j))
	if e = s.transact(ctx, guards); e != nil {
		return safeError()
	}
	return nil
}
func (s *Service) removeDeletedScope(ctx context.Context, p, link row) error {
	if p.State != "deleting" || link.OwnerID != p.OwnerID || link.ProjectID != p.ProjectID {
		return safeError()
	}
	stage, e := s.localStage(ctx)
	if e != nil {
		return e
	}
	a, e := s.get(ctx, s.Table, "U#"+link.OwnerID, "CRED#"+link.LocalAgentID)
	if e != nil {
		return e
	}
	scopes := []Scope{}
	for _, scope := range a.Scopes {
		if scope.ProjectID != p.ProjectID {
			scopes = append(scopes, scope)
		}
	}
	actions := []dt.TransactWriteItem{s.stageGuard(stage), s.projectGuard(p, "deleting"), deleteVersion(s.Table, link)}
	if len(scopes) != len(a.Scopes) || a.ActiveProject == p.ProjectID {
		a.Scopes = scopes
		a.Version++
		if a.ActiveProject == p.ProjectID {
			a.ActiveJob = ""
			a.ActiveProject = ""
		}
		if len(scopes) == 0 {
			a.State = "revoked"
			a.RevokedAt = pointer(stamp(s.now()))
		}
		put, e := s.put(s.Table, a, false)
		if e != nil {
			return e
		}
		actions = append(actions, put)
	} else {
		actions = append(actions, s.rowGuard(a))
	}
	if e = s.transact(ctx, actions); e != nil {
		return safeError()
	}
	return nil
}
