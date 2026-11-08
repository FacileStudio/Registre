package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"testing"
)

func TestVersionIsTheReleaseVersion(t *testing.T) {
	if version == "dev" {
		t.Skip("version is set by -ldflags at build time")
	}
	raw, err := os.ReadFile("../../CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`(?m)^## \[([0-9][^\]]*)\]`).FindSubmatch(raw)
	if match == nil {
		t.Fatal("CHANGELOG.md has no released version heading")
	}
	if want := "v" + string(match[1]); version != want {
		t.Errorf("version = %q, want %q", version, want)
	}
}

func TestVersionHandlerServesTheVersion(t *testing.T) {
	previous := version
	version = "v9.9.9"
	t.Cleanup(func() { version = previous })

	recorder := httptest.NewRecorder()
	versionHandler(recorder, httptest.NewRequest(http.MethodGet, "/version", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != "v9.9.9" || len(body) != 1 {
		t.Errorf("body = %v", body)
	}
}
