package browsersession

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryStore struct {
	mu    sync.Mutex
	gen   string
	rows  map[string]*Record
	rates map[string]int
	fail  bool
}

func newMemory() *memoryStore {
	return &memoryStore{gen: "fixture-generation", rows: map[string]*Record{}, rates: map[string]int{}}
}
func (m *memoryStore) Generation(context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return "", ErrUnavailable
	}
	return m.gen, nil
}
func (m *memoryStore) Get(_ context.Context, key string) (*Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return clone(m.rows[key]), nil
}
func (m *memoryStore) Commit(_ context.Context, gen string, before, after map[string]*Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail || m.gen != gen {
		return ErrUnavailable
	}
	for k, old := range before {
		actual := m.rows[k]
		if old == nil && actual != nil || old != nil && (actual == nil || actual.Version != old.Version) {
			return ErrConflict
		}
	}
	for k, v := range after {
		m.rows[k] = clone(v)
	}
	return nil
}
func (m *memoryStore) Rate(_ context.Context, k string, w int64, limit int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := k + stamp(w)
	if m.rates[key] >= limit {
		return ErrRate
	}
	m.rates[key]++
	return nil
}

type fixtureVerifier struct {
	now         *atomic.Int64
	base        int64
	unavailable atomic.Bool
}

func (v *fixtureVerifier) Verify(_ context.Context, raw string) (Claims, error) {
	if v.unavailable.Load() {
		return Claims{}, ErrUnavailable
	}
	switch raw {
	case "original":
		return Claims{"fixture-user", v.base - 60, v.base, v.base + 900}, nil
	case "rotated":
		return Claims{"fixture-user", v.base - 60, v.now.Load(), v.now.Load() + 900}, nil
	case "foreign":
		return Claims{"foreign-user", v.now.Load() - 60, v.now.Load(), v.now.Load() + 900}, nil
	case "wrong-lineage":
		return Claims{"fixture-user", v.now.Load() - 120, v.now.Load(), v.now.Load() + 900}, nil
	}
	return Claims{}, ErrEnded
}

type fixtureProvider struct {
	rotations   atomic.Int32
	revocations atomic.Int32
	refreshHook func()
	revokeFail  atomic.Bool
	token       string
}

func (p *fixtureProvider) Refresh(context.Context, string) (Tokens, error) {
	p.rotations.Add(1)
	if p.refreshHook != nil {
		p.refreshHook()
	}
	token := p.token
	if token == "" {
		token = "rotated"
	}
	return Tokens{token, "fixture-rotated-refresh"}, nil
}
func (p *fixtureProvider) Revoke(context.Context, string) error {
	p.revocations.Add(1)
	if p.revokeFail.Load() {
		return ErrUnavailable
	}
	return nil
}

type fixture struct {
	s     *Service
	db    *memoryStore
	p     *fixtureProvider
	v     *fixtureVerifier
	clock *atomic.Int64
	in    Bootstrap
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	clock := &atomic.Int64{}
	clock.Store(1800000000)
	db := newMemory()
	p := &fixtureProvider{}
	v := &fixtureVerifier{now: clock, base: clock.Load()}
	s := &Service{Store: db, Provider: p, Verifier: v, Cipher: LocalCipher{Key: make([]byte, 32)}, Origin: "https://ui.invalid", OriginSecret: "fixture-origin-proof", Now: func() time.Time { return time.Unix(clock.Load(), 0) }}
	a, _ := randomProof()
	b, _ := randomProof()
	c, _ := randomProof()
	return &fixture{s, db, p, v, clock, Bootstrap{"fixture-original-refresh", a, b, c}}
}
func (f *fixture) bootstrap(t *testing.T) Result {
	t.Helper()
	v, e := f.s.Bootstrap(context.Background(), "original", f.in)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestLostBootstrapResponseReissuesSameCookieWithoutRotation(t *testing.T) {
	f := newFixture(t)
	first := f.bootstrap(t)
	again := f.bootstrap(t)
	if first != again || f.p.rotations.Load() != 1 {
		t.Fatal("bootstrap retry did not use committed result")
	}
	row, _ := f.db.Get(context.Background(), "S#"+hash(f.in.SessionProof))
	encoded, _ := json.Marshal(row)
	if strings.Contains(string(encoded), "fixture-original-refresh") || strings.Contains(string(encoded), "fixture-rotated-refresh") || strings.Contains(string(encoded), f.in.SessionProof) {
		t.Fatal("credential persisted outside encryption/hash")
	}
	for _, change := range []func(*Bootstrap){func(v *Bootstrap) { v.RefreshToken = "changed-refresh" }, func(v *Bootstrap) { v.CancellationProof, _ = randomProof() }, func(v *Bootstrap) { v.SessionProof, _ = randomProof() }} {
		bad := f.in
		change(&bad)
		if _, e := f.s.Bootstrap(context.Background(), "original", bad); !errors.Is(e, ErrEnded) {
			t.Fatal("changed retry accepted")
		}
	}
	if f.p.rotations.Load() != 1 {
		t.Fatal("changed retry rotated provider")
	}
}
func TestCancellationBeforeAndDuringBootstrapWins(t *testing.T) {
	t.Run("before", func(t *testing.T) {
		f := newFixture(t)
		if e := f.s.Cancel(context.Background(), f.in.AttemptID, f.in.CancellationProof); e != nil {
			t.Fatal(e)
		}
		if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) || f.p.rotations.Load() != 0 {
			t.Fatal("precreation cancellation did not fence creation")
		}
	})
	t.Run("during", func(t *testing.T) {
		f := newFixture(t)
		started, release := make(chan struct{}), make(chan struct{})
		f.p.refreshHook = func() { close(started); <-release }
		done := make(chan error, 1)
		go func() { _, e := f.s.Bootstrap(context.Background(), "original", f.in); done <- e }()
		<-started
		if e := f.s.Cancel(context.Background(), f.in.AttemptID, f.in.CancellationProof); e != nil {
			t.Fatal(e)
		}
		close(release)
		if e := <-done; !errors.Is(e, ErrEnded) {
			t.Fatal("late bootstrap succeeded")
		}
		if _, e := f.s.Metadata(context.Background(), f.in.SessionProof); !errors.Is(e, ErrEnded) {
			t.Fatal("canceled session restored")
		}
	})
}
func TestBootstrapBindsLineageAndRejectsUnavailableWithoutExpiryClaim(t *testing.T) {
	for _, token := range []string{"foreign", "wrong-lineage"} {
		f := newFixture(t)
		f.p.token = token
		if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) {
			t.Fatal("mismatched refreshed identity accepted")
		}
	}
	f := newFixture(t)
	f.v.unavailable.Store(true)
	if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrUnavailable) {
		t.Fatal("JWKS outage misclassified")
	}
}
func TestConcurrentRefreshAndSignoutFencing(t *testing.T) {
	f := newFixture(t)
	v := f.bootstrap(t)
	r, _ := f.db.Get(context.Background(), "S#"+hash(f.in.SessionProof))
	b, _ := f.s.loadBundle(context.Background(), r)
	b.AccessExpiry = f.clock.Load()
	r.Bundle, _ = f.s.seal(context.Background(), r.PK, b)
	f.db.mu.Lock()
	f.db.rows[r.PK] = r
	f.db.mu.Unlock()
	started, release := make(chan struct{}), make(chan struct{})
	f.p.refreshHook = func() { close(started); <-release }
	done := make(chan error, 1)
	go func() {
		_, e := f.s.Token(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding)
		done <- e
	}()
	<-started
	if _, e := f.s.Token(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding); !errors.Is(e, ErrBusy) {
		t.Fatal("waiter did not observe busy state")
	}
	if ok, e := f.s.Logout(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding); e != nil || !ok {
		t.Fatal("logout failed")
	}
	close(release)
	if e := <-done; !errors.Is(e, ErrEnded) {
		t.Fatal("refresh resurrected logout")
	}
	if _, e := f.s.Metadata(context.Background(), f.in.SessionProof); !errors.Is(e, ErrEnded) {
		t.Fatal("revoked session active")
	}
}
func TestExpiryGenerationAndDurableRevocation(t *testing.T) {
	f := newFixture(t)
	v := f.bootstrap(t)
	f.p.revokeFail.Store(true)
	ok, e := f.s.Logout(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding)
	if e != nil || ok {
		t.Fatal("provider failure claimed confirmed")
	}
	if _, e = f.s.Metadata(context.Background(), f.in.SessionProof); !errors.Is(e, ErrEnded) {
		t.Fatal("pending revoke did not deny session")
	}
	f.p.revokeFail.Store(false)
	if ok, e = f.s.Logout(context.Background(), f.in.SessionProof, "", ""); e != nil || !ok {
		t.Fatal("idempotent retry did not confirm revoke")
	}
	g := newFixture(t)
	g.bootstrap(t)
	g.clock.Add(86400)
	if _, e = g.s.Metadata(context.Background(), g.in.SessionProof); !errors.Is(e, ErrEnded) {
		t.Fatal("TTL presence authorized expired session")
	}
	h := newFixture(t)
	h.bootstrap(t)
	h.db.gen = "recovery-next"
	if _, e = h.s.Metadata(context.Background(), h.in.SessionProof); !errors.Is(e, ErrEnded) {
		t.Fatal("old generation remained authorized")
	}
}
func TestNoSessionCapBlocksFreshAuthentication(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 8; i++ {
		f.clock.Add(61)
		f.v.base = f.clock.Load()
		f.in.AttemptID, _ = randomProof()
		f.in.SessionProof, _ = randomProof()
		f.bootstrap(t)
	}
}

func TestFixedLifetimeAndBootstrapRetryWindow(t *testing.T) {
	f := newFixture(t)
	first := f.bootstrap(t)
	f.clock.Add(880)
	updated, e := f.s.Token(context.Background(), f.in.SessionProof, first.CSRFToken, first.SessionBinding)
	if e != nil || updated.SessionExpiresAt != first.SessionExpiresAt || f.p.rotations.Load() != 2 {
		t.Fatal("rotation changed absolute lifetime or failed")
	}
	if _, e = f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) {
		t.Fatal("bootstrap retry outlived five-minute authentication window")
	}
}

func TestLostRotationOutcomeFailsClosedAfterLease(t *testing.T) {
	f := newFixture(t)
	v := f.bootstrap(t)
	r, _ := f.db.Get(context.Background(), "S#"+hash(f.in.SessionProof))
	r.Status = "refreshing"
	r.LeaseUntil = f.clock.Load() - 1
	f.db.mu.Lock()
	f.db.rows[r.PK] = r
	f.db.mu.Unlock()
	if _, e := f.s.Token(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding); !errors.Is(e, ErrEnded) {
		t.Fatal("ambiguous stale refresh lease retried")
	}
	if f.p.rotations.Load() != 1 {
		t.Fatal("uncertain provider refresh replayed")
	}
}
func TestHTTPBoundaryCookieAndStrictJSON(t *testing.T) {
	f := newFixture(t)
	call := func(method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Origin", f.s.Origin)
		r.Header.Set("X-Kurier-Session-Origin", f.s.OriginSecret)
		r.Header.Set("X-Kurier-Session", "1")
		r.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		f.s.ServeHTTP(w, r)
		return w
	}
	raw, _ := json.Marshal(f.in)
	w := call("POST", Path, string(raw), map[string]string{"Authorization": "Bearer original"})
	if w.Code != 201 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("bootstrap HTTP failed")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != f.in.SessionProof || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Domain != "" || cookies[0].MaxAge <= 0 {
		t.Fatal("cookie policy failed")
	}
	retry := call("POST", Path, string(raw), map[string]string{"Authorization": "Bearer original"})
	if retry.Result().Cookies()[0].Value != cookies[0].Value || f.p.rotations.Load() != 1 {
		t.Fatal("lost response did not reissue same cookie")
	}
	for _, headers := range []map[string]string{{"Origin": "https://foreign.invalid"}, {"Origin": "null"}, {"X-Kurier-Session-Origin": "bad"}, {"X-Kurier-Session": ""}, {"Sec-Fetch-Mode": "navigate"}, {"Sec-Fetch-Site": "cross-site"}} {
		if got := call("GET", Path, "", headers); got.Code != 403 {
			t.Fatal("origin/CSRF boundary failed")
		}
	}
	for _, body := range []string{`{"attemptId":"a","attemptId":"b"}`, `{} {}`, `{"unknown":"field"}`, `{"refreshToken":{}}`, string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'})} {
		if got := call("POST", Path, body, map[string]string{"Authorization": "Bearer original"}); got.Code != 400 {
			t.Fatal("malformed JSON accepted")
		}
	}
	if got := call("DELETE", Path, "", nil); got.Code != 200 || got.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("absent session safe clear failed")
	}
}
func TestCipherBindsSessionAndRejectsTampering(t *testing.T) {
	c := LocalCipher{Key: make([]byte, 32)}
	sealed, e := c.Seal(context.Background(), "one", []byte("fixture"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Open(context.Background(), "two", sealed); e == nil {
		t.Fatal("cross-session decryption accepted")
	}
	sealed[len(sealed)-1] ^= 1
	if _, e = c.Open(context.Background(), "one", sealed); e == nil {
		t.Fatal("tampering accepted")
	}
}

type verifierFunc func(context.Context, string) (Claims, error)

func (f verifierFunc) Verify(ctx context.Context, v string) (Claims, error) { return f(ctx, v) }

type hookCipher struct {
	Cipher
	afterSeal, afterOpen func()
}

func (c hookCipher) Seal(ctx context.Context, id string, raw []byte) ([]byte, error) {
	v, e := c.Cipher.Seal(ctx, id, raw)
	if c.afterSeal != nil {
		c.afterSeal()
	}
	return v, e
}
func (c hookCipher) Open(ctx context.Context, id string, raw []byte) ([]byte, error) {
	v, e := c.Cipher.Open(ctx, id, raw)
	if c.afterOpen != nil {
		c.afterOpen()
	}
	return v, e
}

func TestFinalDeadlineChecksAfterProviderAndKMS(t *testing.T) {
	t.Run("bootstrap session deadline during provider", func(t *testing.T) {
		f := newFixture(t)
		start := f.clock.Load()
		f.s.Verifier = verifierFunc(func(context.Context, string) (Claims, error) {
			return Claims{"fixture-user", start - 86400 + 2, start, start + 900}, nil
		})
		f.p.refreshHook = func() { f.clock.Add(3) }
		if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) {
			t.Fatal("bootstrap issued expired session")
		}
	})
	t.Run("bootstrap retry window during provider", func(t *testing.T) {
		f := newFixture(t)
		start := f.clock.Load()
		f.s.Verifier = verifierFunc(func(context.Context, string) (Claims, error) {
			return Claims{"fixture-user", start - 400, start - 299, start + 900}, nil
		})
		f.p.refreshHook = func() { f.clock.Add(2) }
		if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) {
			t.Fatal("bootstrap exceeded retry freshness window")
		}
	})
	t.Run("bootstrap access expiry during seal", func(t *testing.T) {
		f := newFixture(t)
		start := f.clock.Load()
		f.s.Verifier = verifierFunc(func(context.Context, string) (Claims, error) {
			return Claims{"fixture-user", start - 60, start, start + 2}, nil
		})
		f.s.Cipher = hookCipher{Cipher: f.s.Cipher, afterSeal: func() { f.clock.Add(3) }}
		if _, e := f.s.Bootstrap(context.Background(), "original", f.in); !errors.Is(e, ErrEnded) {
			t.Fatal("bootstrap returned expired access token")
		}
	})
	t.Run("refresh absolute deadline during provider", func(t *testing.T) {
		f := newFixture(t)
		v := f.bootstrap(t)
		r, _ := f.db.Get(context.Background(), "S#"+hash(f.in.SessionProof))
		b, _ := f.s.loadBundle(context.Background(), r)
		b.AccessExpiry = f.clock.Load()
		r.Expires = f.clock.Load() + 2
		r.Bundle, _ = f.s.seal(context.Background(), r.PK, b)
		f.db.mu.Lock()
		f.db.rows[r.PK] = r
		f.db.mu.Unlock()
		f.p.refreshHook = func() { f.clock.Add(3) }
		if _, e := f.s.Token(context.Background(), f.in.SessionProof, v.CSRFToken, v.SessionBinding); !errors.Is(e, ErrEnded) {
			t.Fatal("rotation issued expired session")
		}
	})
	t.Run("final output rechecks deadline after decrypt", func(t *testing.T) {
		f := newFixture(t)
		f.bootstrap(t)
		r, _ := f.db.Get(context.Background(), "S#"+hash(f.in.SessionProof))
		r.Expires = f.clock.Load() + 2
		f.s.Cipher = hookCipher{Cipher: f.s.Cipher, afterOpen: func() { f.clock.Add(3) }}
		if _, e := f.s.output(context.Background(), r, true); !errors.Is(e, ErrEnded) {
			t.Fatal("output exposed token after session deadline")
		}
	})
}

func TestAccountReplacementBindingPrecedesOldCSRF(t *testing.T) {
	f := newFixture(t)
	old := f.bootstrap(t)
	replacement := f.in
	replacement.AttemptID, _ = randomProof()
	replacement.SessionProof, _ = randomProof()
	replacement.CancellationProof, _ = randomProof()
	next, e := f.s.Bootstrap(context.Background(), "original", replacement)
	if e != nil {
		t.Fatal(e)
	}
	for _, method := range []string{"POST", "DELETE"} {
		path, body := Path+"/token", "{}"
		if method == "DELETE" {
			path = Path
			body = ""
		}
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Kurier-Session-Origin", f.s.OriginSecret)
		r.Header.Set("X-Kurier-Session", "1")
		r.Header.Set("Origin", f.s.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Kurier-CSRF", old.CSRFToken)
		r.Header.Set("X-Kurier-Session-Binding", old.SessionBinding)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: replacement.SessionProof})
		w := httptest.NewRecorder()
		f.s.ServeHTTP(w, r)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "session_changed") {
			t.Fatal("replacement cookie did not trigger binding revalidation")
		}
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("X-Kurier-Session-Origin", f.s.OriginSecret)
		r.Header.Set("X-Kurier-Session", "1")
		r.Header.Set("Origin", f.s.Origin)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("X-Kurier-CSRF", old.CSRFToken)
		r.Header.Set("X-Kurier-Session-Binding", next.SessionBinding)
		r.AddCookie(&http.Cookie{Name: CookieName, Value: replacement.SessionProof})
		w = httptest.NewRecorder()
		f.s.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("matching binding with wrong CSRF not forbidden")
		}
	}
}

type scanMemory struct{ *memoryStore }

func (m scanMemory) Scan(_ context.Context, cursor string) ([]Record, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := []string{}
	for k, r := range m.rows {
		if r.Kind == "session" && k > cursor {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	rows := []Record{}
	for _, k := range keys {
		rows = append(rows, *clone(m.rows[k]))
	}
	return rows, "", nil
}
func TestMaintenanceErasesExpiredBundleAndBoundsRetryWork(t *testing.T) {
	f := newFixture(t)
	f.s.Store = scanMemory{f.db}
	f.p.revokeFail.Store(true)
	for i := 0; i < 6; i++ {
		id := string(rune('a' + i))
		r := &Record{PK: "S#" + id, Kind: "session", Version: 1, Generation: f.db.gen, Status: "revoked", Expires: f.clock.Load() + 600}
		r.Bundle, _ = f.s.seal(context.Background(), r.PK, bundle{"fixture-access", "fixture-refresh", f.clock.Load() + 900})
		f.db.rows[r.PK] = r
	}
	expired := clone(f.db.rows["S#a"])
	expired.PK = "S#0"
	expired.Expires = f.clock.Load()
	f.db.rows[expired.PK] = expired
	if e := f.s.Maintain(context.Background()); e != nil {
		t.Fatal(e)
	}
	if len(f.db.rows["S#0"].Bundle) != 0 || f.p.revocations.Load() != 2 {
		t.Fatal("expired credentials retained or first tick exceeded work budget")
	}
	if e := f.s.Maintain(context.Background()); e != nil {
		t.Fatal(e)
	}
	if f.p.revocations.Load() != 5 {
		t.Fatal("cleanup cursor failed to advance bounded retry work")
	}
}
