package env

import (
	"testing"
	"time"
)

func TestTheHardeningDefaultsCapThePoolAndTheAddressBudgets(t *testing.T) {
	base(t)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBMaxOpen != 40 || cfg.DBMaxIdle != 10 || cfg.DBMaxLifetime != 30*time.Minute {
		t.Errorf("pool = %d/%d/%s, want 40/10/30m", cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBMaxLifetime)
	}
	if cfg.LoginRate != 1 || cfg.LoginBurst != 10 || cfg.TokenRate != 1 || cfg.TokenBurst != 10 {
		t.Errorf("budgets = %v/%d %v/%d, want 1/10 for both", cfg.LoginRate, cfg.LoginBurst, cfg.TokenRate, cfg.TokenBurst)
	}
	if cfg.VerifyWait != 3*time.Second {
		t.Errorf("verify wait = %s, want 3s", cfg.VerifyWait)
	}
}

func TestTheHardeningVariablesOverrideTheDefaults(t *testing.T) {
	base(t)
	t.Setenv("REGISTRE_DB_MAX_OPEN_CONNS", "20")
	t.Setenv("REGISTRE_DB_MAX_IDLE_CONNS", "50")
	t.Setenv("REGISTRE_LOGIN_RATE", "0.5")
	t.Setenv("REGISTRE_TOKEN_BURST", "3")
	t.Setenv("REGISTRE_VERIFY_QUEUE_WAIT", "500ms")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBMaxOpen != 20 || cfg.DBMaxIdle != 20 {
		t.Errorf("pool = %d/%d, want idle clamped to open: 20/20", cfg.DBMaxOpen, cfg.DBMaxIdle)
	}
	if cfg.LoginRate != 0.5 || cfg.TokenBurst != 3 || cfg.VerifyWait != 500*time.Millisecond {
		t.Errorf("overrides not read: %+v", cfg.Hardening)
	}
}

func TestNonsenseHardeningValuesFailTheBoot(t *testing.T) {
	for key, value := range map[string]string{
		"REGISTRE_DB_MAX_OPEN_CONNS": "0",
		"REGISTRE_LOGIN_RATE":        "0",
		"REGISTRE_TOKEN_RATE":        "fast",
		"REGISTRE_LOGIN_BURST":       "0",
		"REGISTRE_VERIFY_QUEUE_WAIT": "soon",
	} {
		base(t)
		t.Setenv(key, value)
		if _, err := Load(); err == nil {
			t.Errorf("%s=%q was accepted", key, value)
		}
	}
}

func TestTheVerifyConcurrencySurvivesTheHardeningLoad(t *testing.T) {
	base(t)
	t.Setenv("REGISTRE_VERIFY_CONCURRENCY", "7")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VerifyConcurrency != 7 {
		t.Fatalf("VerifyConcurrency = %d, want 7", cfg.VerifyConcurrency)
	}
}
