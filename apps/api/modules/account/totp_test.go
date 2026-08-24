package account

import (
	"context"
	"net/http"
	"testing"
)

func TestDisablingAndRegeneratingWantThePasswordAndAProof(t *testing.T) {
	f := newFixture(t)
	codes := f.enrol(t)
	path := "/api/account/totp/disable"
	regenerate := "/api/account/totp/recovery-codes"

	response, _ := f.send(t, http.MethodPost, regenerate, map[string]any{
		"password": "wrong", "code": codes[0],
	})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("regenerating with a wrong password answered %d, want 401", response.StatusCode)
	}

	if err := f.factors.RedeemRecoveryCode(context.Background(), f.user, codes[0]); err != nil {
		t.Fatal("a wrong password spent the recovery code presented with it: " +
			"anyone holding a stolen session could burn all ten by guessing")
	}
	codes = codes[1:]

	response, _ = f.send(t, http.MethodPost, regenerate, map[string]any{"password": holderPassword})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("regenerating with a password and no factor proof answered %d, want 401: "+
			"a fresh set of codes handed out for a password alone is the way around disable, "+
			"which asks for a factor proof that this call would have just minted",
			response.StatusCode)
	}

	response, fresh := f.send(t, http.MethodPost, regenerate, map[string]any{
		"password": holderPassword, "code": codes[0],
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("regenerating answered %d: %v", response.StatusCode, fresh)
	}
	if err := f.factors.RedeemRecoveryCode(context.Background(), f.user, codes[0]); err == nil {
		t.Fatal("a replaced recovery code still opens the account")
	}
	codes = recoveryCodes(t, fresh)

	response, _ = f.send(t, http.MethodPost, path, map[string]any{"code": codes[0]})
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("disabling with no password answered %d, want 401", response.StatusCode)
	}
	if !f.reload(t).TOTPEnrolled() {
		t.Fatal("the refused request removed the factor anyway")
	}

	response, body := f.send(t, http.MethodPost, path, map[string]any{
		"password": holderPassword, "code": codes[1],
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("disabling answered %d: %v", response.StatusCode, body)
	}
	if f.reload(t).TOTPEnrolled() {
		t.Fatal("the factor survived being disabled")
	}
	left, err := f.factors.RemainingRecoveryCodes(context.Background(), f.user)
	if err != nil {
		t.Fatalf("counting the recovery codes: %v", err)
	}
	if left != 0 {
		t.Fatalf("%d recovery codes outlived the factor they open", left)
	}
}
