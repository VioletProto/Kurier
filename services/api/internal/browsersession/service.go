package browsersession

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type Bootstrap struct {
	RefreshToken      string `json:"refreshToken"`
	AttemptID         string `json:"attemptId"`
	CancellationProof string `json:"cancellationProof"`
	SessionProof      string `json:"sessionProof"`
}
type Result struct {
	AccessToken      string `json:"accessToken,omitempty"`
	AccessExpiresAt  string `json:"accessExpiresAt,omitempty"`
	SessionExpiresAt string `json:"sessionExpiresAt"`
	CSRFToken        string `json:"csrfToken,omitempty"`
	SessionBinding   string `json:"sessionBinding"`
}

func stamp(t int64) string { return time.Unix(t, 0).UTC().Format(time.RFC3339) }
func (s *Service) loadBundle(ctx context.Context, r *Record) (bundle, error) {
	var b bundle
	raw, e := s.Cipher.Open(ctx, r.PK, r.Bundle)
	if e != nil {
		return b, ErrUnavailable
	}
	defer clear(raw)
	if json.Unmarshal(raw, &b) != nil || b.Access == "" || b.Refresh == "" {
		return b, ErrUnavailable
	}
	return b, nil
}
func (s *Service) seal(ctx context.Context, key string, b bundle) ([]byte, error) {
	raw, e := json.Marshal(b)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer clear(raw)
	sealed, e := s.Cipher.Seal(ctx, key, raw)
	if e != nil {
		return nil, ErrUnavailable
	}
	return sealed, nil
}
func (s *Service) output(ctx context.Context, r *Record, token bool) (Result, error) {
	if r.Expires <= s.now().Unix() {
		return Result{}, ErrEnded
	}
	v := Result{SessionExpiresAt: stamp(r.Expires), SessionBinding: r.Binding, CSRFToken: r.CSRF}
	if token {
		b, e := s.loadBundle(ctx, r)
		if e != nil {
			return Result{}, e
		}
		v.AccessToken = b.Access
		v.AccessExpiresAt = stamp(b.AccessExpiry)
		if b.AccessExpiry <= s.now().Unix() {
			return Result{}, ErrEnded
		}
	}
	if r.Expires <= s.now().Unix() {
		return Result{}, ErrEnded
	}
	return v, nil
}
func (s *Service) active(ctx context.Context, proof string) (*Record, error) {
	if !validProof(proof) {
		return nil, ErrEnded
	}
	gen, e := s.Store.Generation(ctx)
	if e != nil {
		return nil, ErrUnavailable
	}
	r, e := s.Store.Get(ctx, "S#"+hash(proof))
	if e != nil {
		return nil, ErrUnavailable
	}
	if r == nil || r.Generation != gen || r.Kind != "session" || r.Status == "revoked" || r.Expires <= s.now().Unix() {
		return nil, ErrEnded
	}
	if r.Status != "active" && r.Status != "refreshing" {
		return nil, ErrEnded
	}
	return r, nil
}
func (s *Service) Metadata(ctx context.Context, proof string) (Result, error) {
	r, e := s.active(ctx, proof)
	if e != nil {
		return Result{}, e
	}
	return s.output(ctx, r, false)
}

func (s *Service) Bootstrap(ctx context.Context, access string, in Bootstrap) (Result, error) {
	if !validProof(in.AttemptID) || !validProof(in.CancellationProof) || !validProof(in.SessionProof) || len(in.RefreshToken) == 0 || len(in.RefreshToken) > 8192 {
		return Result{}, ErrEnded
	}
	c, e := s.Verifier.Verify(ctx, access)
	if e != nil {
		return Result{}, providerError(e)
	}
	now := s.now().Unix()
	if e != nil || c.AuthTime <= 0 || c.AuthTime > now || c.IssuedAt > now || now-c.IssuedAt > 300 || c.ExpiresAt <= now || c.AuthTime+86400 <= now {
		return Result{}, ErrEnded
	}
	gen, e := s.Store.Generation(ctx)
	if e != nil {
		return Result{}, ErrUnavailable
	}
	ak, sk := "A#"+hash(in.AttemptID), "S#"+hash(in.SessionProof)
	requestHash := hash(access + "\x00" + in.RefreshToken)
	a, e := s.Store.Get(ctx, ak)
	if e != nil {
		return Result{}, ErrUnavailable
	}
	if a != nil {
		if a.Status == "pending" && a.Generation == gen && a.Expires > now && same(a.RequestHash, requestHash) && same(a.CancelHash, hash(in.CancellationProof)) && a.SessionKey == sk && a.Subject == c.Subject && a.AuthTime == c.AuthTime && a.IssuedAt == c.IssuedAt {
			return Result{}, ErrBusy
		}
		if a.Generation != gen || a.Expires <= now || a.Status != "complete" || !same(a.RequestHash, requestHash) || !same(a.CancelHash, hash(in.CancellationProof)) || a.SessionKey != sk || a.Subject != c.Subject || a.AuthTime != c.AuthTime || a.IssuedAt != c.IssuedAt {
			return Result{}, ErrEnded
		}
		r, e := s.active(ctx, in.SessionProof)
		if e != nil {
			return Result{}, e
		}
		return s.output(ctx, r, true)
	}
	if e = s.Store.Rate(ctx, "bootstrap#"+hash(c.Subject), now/60, 6); e != nil {
		return Result{}, e
	}
	csrf, e := randomProof()
	if e != nil {
		return Result{}, ErrUnavailable
	}
	binding, e := randomProof()
	if e != nil {
		return Result{}, ErrUnavailable
	}
	a = &Record{PK: ak, Kind: "attempt", Version: 1, Status: "pending", Generation: gen, Subject: c.Subject, AuthTime: c.AuthTime, IssuedAt: c.IssuedAt, Expires: c.IssuedAt + 300, TTL: c.IssuedAt + 900, SessionKey: sk, CancelHash: hash(in.CancellationProof)}
	a.RequestHash = requestHash
	r := &Record{PK: sk, Kind: "session", Version: 1, Status: "pending", Generation: gen, Subject: c.Subject, AuthTime: c.AuthTime, Expires: c.AuthTime + 86400, TTL: c.AuthTime + 86400, AttemptKey: ak, Binding: binding, CSRF: csrf, LeaseUntil: now + 10}
	if e = s.Store.Commit(ctx, gen, map[string]*Record{ak: nil, sk: nil}, map[string]*Record{ak: a, sk: r}); e != nil {
		if errors.Is(e, ErrConflict) {
			return Result{}, ErrBusy
		}
		return Result{}, ErrUnavailable
	}
	tokens, e := s.Provider.Refresh(ctx, in.RefreshToken)
	if e != nil {
		s.endPending(ctx, a, r)
		return Result{}, providerError(e)
	}
	rotated, e := s.Verifier.Verify(ctx, tokens.Access)
	if errors.Is(e, ErrUnavailable) {
		_ = s.Provider.Revoke(ctx, tokens.Refresh)
		s.endPending(ctx, a, r)
		return Result{}, ErrUnavailable
	}
	if e != nil || rotated.Subject != c.Subject || rotated.AuthTime != c.AuthTime || rotated.ExpiresAt <= s.now().Unix() || tokens.Refresh == "" {
		_ = s.Provider.Revoke(ctx, tokens.Refresh)
		s.endPending(ctx, a, r)
		return Result{}, ErrEnded
	}
	sealed, e := s.seal(ctx, sk, bundle{tokens.Access, tokens.Refresh, rotated.ExpiresAt})
	if e != nil {
		_ = s.Provider.Revoke(ctx, tokens.Refresh)
		s.endPending(ctx, a, r)
		return Result{}, e
	}
	nextA, nextR := clone(a), clone(r)
	nextA.Version++
	nextA.Status = "complete"
	nextR.Version++
	nextR.Status = "active"
	nextR.LeaseUntil = 0
	nextR.Bundle = sealed
	if s.now().Unix() >= r.LeaseUntil || s.now().Unix() >= a.Expires || s.now().Unix() >= r.Expires || rotated.ExpiresAt <= s.now().Unix() {
		_ = s.Provider.Revoke(ctx, tokens.Refresh)
		s.endPending(ctx, a, r)
		return Result{}, ErrEnded
	}
	if e = s.Store.Commit(ctx, gen, map[string]*Record{ak: a, sk: r}, map[string]*Record{ak: nextA, sk: nextR}); e != nil {
		_ = s.Provider.Revoke(ctx, tokens.Refresh)
		return Result{}, ErrEnded
	}
	return s.output(ctx, nextR, true)
}
func providerError(e error) error {
	if errors.Is(e, ErrEnded) {
		return ErrEnded
	}
	return ErrUnavailable
}
func (s *Service) endPending(ctx context.Context, a, r *Record) {
	na, nr := clone(a), clone(r)
	na.Version++
	nr.Version++
	na.Status = "canceled"
	nr.Status = "revoked"
	nr.Bundle = nil
	_ = s.Store.Commit(ctx, a.Generation, map[string]*Record{a.PK: a, r.PK: r}, map[string]*Record{a.PK: na, r.PK: nr})
}
func (s *Service) Cancel(ctx context.Context, id, proof string) error {
	if !validProof(id) || !validProof(proof) {
		return ErrEnded
	}
	gen, e := s.Store.Generation(ctx)
	if e != nil {
		return ErrUnavailable
	}
	key := "A#" + hash(id)
	for tries := 0; tries < 3; tries++ {
		a, e := s.Store.Get(ctx, key)
		if e != nil {
			return ErrUnavailable
		}
		if a != nil && !same(a.CancelHash, hash(proof)) {
			return ErrEnded
		}
		if a != nil && a.Status == "canceled" {
			return nil
		}
		if a == nil {
			v := &Record{PK: key, Kind: "attempt", Version: 1, Generation: gen, Status: "canceled", CancelHash: hash(proof), Expires: s.now().Add(5 * time.Minute).Unix(), TTL: s.now().Add(15 * time.Minute).Unix()}
			e = s.Store.Commit(ctx, gen, map[string]*Record{key: nil}, map[string]*Record{key: v})
			if errors.Is(e, ErrConflict) {
				continue
			}
			return e
		}
		na := clone(a)
		na.Version++
		na.Status = "canceled"
		before := map[string]*Record{key: a}
		after := map[string]*Record{key: na}
		var revoked *Record
		if a.SessionKey != "" {
			r, e := s.Store.Get(ctx, a.SessionKey)
			if e != nil {
				return ErrUnavailable
			}
			if r != nil {
				revoked = clone(r)
				revoked.Version++
				revoked.Status = "revoked"
				before[r.PK] = r
				after[r.PK] = revoked
			}
		}
		e = s.Store.Commit(ctx, gen, before, after)
		if errors.Is(e, ErrConflict) {
			continue
		}
		if e != nil {
			return ErrUnavailable
		}
		if revoked != nil {
			_, _ = s.revoke(ctx, revoked)
		}
		return nil
	}
	return ErrConflict
}
func (s *Service) Token(ctx context.Context, proof, csrf, binding string) (Result, error) {
	r, e := s.active(ctx, proof)
	if e != nil {
		return Result{}, e
	}
	if !same(binding, r.Binding) {
		return Result{}, ErrConflict
	}
	if !same(csrf, r.CSRF) {
		return Result{}, ErrForbidden
	}
	now := s.now().Unix()
	if e = s.Store.Rate(ctx, "token#"+r.PK, now/60, 30); e != nil {
		return Result{}, e
	}
	if r.Status == "refreshing" {
		if r.LeaseUntil > now {
			return Result{}, ErrBusy
		}
		nr := clone(r)
		nr.Status = "revoked"
		nr.Version++
		_ = s.Store.Commit(ctx, r.Generation, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: nr})
		return Result{}, ErrEnded
	}
	b, e := s.loadBundle(ctx, r)
	if e != nil {
		return Result{}, e
	}
	if b.AccessExpiry > now+30 {
		return s.output(ctx, r, true)
	}
	lease := clone(r)
	lease.Version++
	lease.Status = "refreshing"
	lease.LeaseUntil = now + 10
	if e = s.Store.Commit(ctx, r.Generation, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: lease}); e != nil {
		if errors.Is(e, ErrConflict) {
			return Result{}, ErrBusy
		}
		return Result{}, e
	}
	t, e := s.Provider.Refresh(ctx, b.Refresh)
	if e != nil {
		s.failLease(ctx, lease)
		return Result{}, providerError(e)
	}
	c, e := s.Verifier.Verify(ctx, t.Access)
	if errors.Is(e, ErrUnavailable) {
		_ = s.Provider.Revoke(ctx, t.Refresh)
		s.failLease(ctx, lease)
		return Result{}, ErrUnavailable
	}
	if e != nil || c.Subject != r.Subject || c.AuthTime != r.AuthTime || c.ExpiresAt <= s.now().Unix() || t.Refresh == "" {
		_ = s.Provider.Revoke(ctx, t.Refresh)
		s.failLease(ctx, lease)
		return Result{}, ErrEnded
	}
	sealed, e := s.seal(ctx, r.PK, bundle{t.Access, t.Refresh, c.ExpiresAt})
	if e != nil {
		_ = s.Provider.Revoke(ctx, t.Refresh)
		s.failLease(ctx, lease)
		return Result{}, e
	}
	next := clone(lease)
	next.Version++
	next.Status = "active"
	next.LeaseUntil = 0
	next.Bundle = sealed
	if s.now().Unix() >= lease.LeaseUntil || s.now().Unix() >= r.Expires || c.ExpiresAt <= s.now().Unix() {
		_ = s.Provider.Revoke(ctx, t.Refresh)
		s.failLease(ctx, lease)
		return Result{}, ErrEnded
	}
	if e = s.Store.Commit(ctx, r.Generation, map[string]*Record{r.PK: lease}, map[string]*Record{r.PK: next}); e != nil {
		_ = s.Provider.Revoke(ctx, t.Refresh)
		return Result{}, ErrEnded
	}
	return s.output(ctx, next, true)
}
func (s *Service) failLease(ctx context.Context, r *Record) {
	next := clone(r)
	next.Version++
	next.Status = "revoked"
	_ = s.Store.Commit(ctx, r.Generation, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: next})
	_, _ = s.revoke(ctx, next)
}
func (s *Service) Logout(ctx context.Context, proof, csrf, binding string) (bool, error) {
	if !validProof(proof) {
		return true, nil
	}
	gen, e := s.Store.Generation(ctx)
	if e != nil {
		return false, ErrUnavailable
	}
	r, e := s.Store.Get(ctx, "S#"+hash(proof))
	if e != nil {
		return false, ErrUnavailable
	}
	if r == nil || r.Generation != gen || r.Expires <= s.now().Unix() {
		return true, nil
	}
	if r.Status == "revoked" {
		return s.revoke(ctx, r)
	}
	if !same(binding, r.Binding) {
		return false, ErrConflict
	}
	if !same(csrf, r.CSRF) {
		return false, ErrForbidden
	}
	next := clone(r)
	next.Version++
	next.Status = "revoked"
	if e = s.Store.Commit(ctx, gen, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: next}); e != nil {
		return false, e
	}
	return s.revoke(ctx, next)
}
func (s *Service) revoke(ctx context.Context, r *Record) (bool, error) {
	if len(r.Bundle) == 0 {
		return true, nil
	}
	b, e := s.loadBundle(ctx, r)
	if e != nil {
		return false, nil
	}
	if s.Provider.Revoke(ctx, b.Refresh) != nil {
		return false, nil
	}
	next := clone(r)
	next.Version++
	next.Bundle = nil
	_ = s.Store.Commit(ctx, r.Generation, map[string]*Record{r.PK: r}, map[string]*Record{r.PK: next})
	return true, nil
}
