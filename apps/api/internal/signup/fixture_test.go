package signup

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/FacileStudio/Registre/apps/api/internal/consent"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/passkeys"
	"github.com/FacileStudio/Registre/apps/api/internal/pwned"
	"github.com/FacileStudio/Registre/apps/api/internal/testdb"

	altcha "github.com/altcha-org/altcha-lib-go"
	"gorm.io/gorm"
)

const testKey = "an-altcha-hmac-key-of-at-least-32-bytes"

type mail struct{ to, subject, html string }

type inbox struct {
	mu   sync.Mutex
	sent []mail
	fail error
}

func (i *inbox) Send(_ context.Context, to, subject, html string) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fail != nil {
		return i.fail
	}
	i.sent = append(i.sent, mail{to, subject, html})
	return nil
}

func (i *inbox) failWith(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.fail = err
}

func (i *inbox) last(t *testing.T) mail {
	t.Helper()
	i.mu.Lock()
	defer i.mu.Unlock()
	if len(i.sent) == 0 {
		t.Fatal("no mail was sent")
	}
	return i.sent[len(i.sent)-1]
}

type world struct {
	svc   *Service
	db    *gorm.DB
	inbox *inbox
}

// newWorld builds the flow over a fresh schema. breached is the HIBP fake's
// answer; nil means HIBP is unreachable.
func newWorld(t *testing.T, breached map[string]bool) world {
	t.Helper()
	url, ok := testdb.URL()
	if !ok {
		testdb.Announce()
		t.Skip(testdb.SkipReason())
	}
	db, err := testdb.Open(url)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := testdb.Truncate(db); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	logs := slog.New(slog.NewTextHandler(io.Discard, nil))
	keys, err := passkeys.New(db, passkeys.Config{RPID: "localhost", Origins: []string{"http://localhost:8080"}}, logs)
	if err != nil {
		t.Fatalf("passkeys: %v", err)
	}
	mails := &inbox{}
	svc := New(Deps{
		DB: db, Verifier: credentials.New(2), Consents: consent.New(db, []byte("consent-key")),
		Passkeys: keys, Pwned: pwned.NewAt(hibp(t, breached), logs), Mailer: mails,
	}, testKey, "https://sso.example.com")
	return world{svc: svc, db: db, inbox: mails}
}

// hibp serves the range API for the given passwords, or a dead URL.
func hibp(t *testing.T, breached map[string]bool) string {
	t.Helper()
	if breached == nil {
		dead := httptest.NewServer(http.NotFoundHandler())
		dead.Close()
		return dead.URL + "/range/"
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := strings.TrimPrefix(r.URL.Path, "/range/")
		for password := range breached {
			if full := sha1Hex(password); strings.HasPrefix(full, prefix) {
				_, _ = io.WriteString(w, full[5:]+":42\r\n")
			}
		}
		_, _ = io.WriteString(w, "0000000000000000000000000000000000A:0\r\n")
	}))
	t.Cleanup(server.Close)
	return server.URL + "/range/"
}

// solve answers a fresh challenge the way the browser widget does.
func solve(t *testing.T, svc *Service) string {
	t.Helper()
	challenge, err := svc.Challenge()
	if err != nil {
		t.Fatalf("Challenge: %v", err)
	}
	solution, err := altcha.SolveChallenge(challenge.Challenge, challenge.Salt, altcha.SHA256, int(challenge.MaxNumber), 0, nil)
	if err != nil || solution == nil {
		t.Fatalf("solving: %v", err)
	}
	payload, _ := json.Marshal(altcha.Payload{
		Algorithm: challenge.Algorithm, Challenge: challenge.Challenge, Number: int64(solution.Number),
		Salt: challenge.Salt, Signature: challenge.Signature,
	})
	return base64.StdEncoding.EncodeToString(payload)
}

// tokenIn extracts the link token from a verification mail.
func tokenIn(t *testing.T, html string) string {
	t.Helper()
	_, after, found := strings.Cut(html, "token=")
	if !found {
		t.Fatalf("no token in %s", html)
	}
	token, _, _ := strings.Cut(after, `"`)
	return token
}
