package metadoc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Cache bounds. A document is re-fetched once its HTTP cache lifetime runs
// out, held at least MinCache so a client cannot make registre fetch on every
// request, and at most MaxCache so a changed document is noticed within a day.
const (
	MinCache     = 5 * time.Minute
	DefaultCache = time.Hour
	MaxCache     = 24 * time.Hour
	fetchTimeout = 5 * time.Second
)

// Fetcher reads client ID metadata documents. Its client must dial through
// an SSRF guard and follow no redirect: the document must answer at the
// exact URL it names.
type Fetcher struct {
	Client *http.Client
	Now    func() time.Time
}

// IsDocumentURL reports whether clientID is shaped like a client ID metadata
// document URL: https, a host, a path other than "/", and no query, fragment,
// credentials or dot segments (draft-ietf-oauth-client-id-metadata-document §3).
func IsDocumentURL(clientID string) bool {
	parsed, err := url.Parse(clientID)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return false
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path == "" || parsed.Path == "/" {
		return false
	}
	for _, segment := range strings.Split(parsed.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return parsed.String() == clientID
}

// Fetch reads and validates the document at clientID and returns it with
// the time it stops being fresh. The document's client_id must equal the URL
// byte for byte, or anyone could host a document claiming another client.
func (f *Fetcher) Fetch(ctx context.Context, clientID string) (Metadata, time.Time, error) {
	if !IsDocumentURL(clientID) {
		return Metadata{}, time.Time{}, invalid("client_id is not a metadata document URL")
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	response, err := f.get(ctx, clientID)
	if err != nil {
		return Metadata{}, time.Time{}, err
	}
	defer func() { _ = response.Body.Close() }()
	document, err := decode(response)
	if err != nil {
		return Metadata{}, time.Time{}, err
	}
	if document.ClientID != clientID {
		return Metadata{}, time.Time{}, invalid("the document's client_id is not the URL it was fetched from")
	}
	validated, err := Validate(document)
	if err != nil {
		return Metadata{}, time.Time{}, err
	}
	return validated, f.now().Add(freshness(response.Header.Get("Cache-Control"))), nil
}

// get performs the request.
func (f *Fetcher) get(ctx context.Context, clientID string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
	if err != nil {
		return nil, invalid("client_id is not a fetchable URL")
	}
	request.Header.Set("Accept", "application/json")
	response, err := f.Client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("metadoc: fetch the document: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("metadoc: the document answered %d", response.StatusCode)
	}
	return response, nil
}

// decode reads at most MaxDocument bytes of JSON.
func decode(response *http.Response) (Metadata, error) {
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxDocument+1))
	if err != nil {
		return Metadata{}, fmt.Errorf("metadoc: read the document: %w", err)
	}
	if len(body) > MaxDocument {
		return Metadata{}, invalid("the document is larger than 16 KiB")
	}
	var document Metadata
	if err := json.Unmarshal(body, &document); err != nil {
		return Metadata{}, errors.Join(ErrInvalid, fmt.Errorf("the document is not JSON client metadata: %w", err))
	}
	return document, nil
}

// freshness is the document's cache lifetime from Cache-Control max-age,
// clamped to [MinCache, MaxCache]. no-store and no-cache count as zero.
func freshness(header string) time.Duration {
	lifetime := DefaultCache
	for _, directive := range strings.Split(header, ",") {
		directive = strings.ToLower(strings.TrimSpace(directive))
		switch {
		case directive == "no-store" || directive == "no-cache":
			return MinCache
		case strings.HasPrefix(directive, "max-age="):
			if seconds, err := strconv.Atoi(strings.TrimPrefix(directive, "max-age=")); err == nil {
				lifetime = time.Duration(seconds) * time.Second
			}
		}
	}
	return min(max(lifetime, MinCache), MaxCache)
}

func (f *Fetcher) now() time.Time {
	if f.Now == nil {
		return time.Now().UTC()
	}
	return f.Now()
}
