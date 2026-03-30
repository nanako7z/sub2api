package service

import "testing"

func TestAccountIsTLSFingerprintEnabled(t *testing.T) {
	t.Run("enabled when anthropic oauth switch is on", func(t *testing.T) {
		acc := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"enable_tls_fingerprint": true,
			},
		}
		if !acc.IsTLSFingerprintEnabled() {
			t.Fatal("expected tls fingerprint to be enabled")
		}
	})

	t.Run("disabled when anthropic oauth switch is off", func(t *testing.T) {
		acc := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"enable_tls_fingerprint": false,
			},
		}
		if acc.IsTLSFingerprintEnabled() {
			t.Fatal("expected tls fingerprint to be disabled")
		}
	})

	t.Run("disabled when anthropic oauth switch missing", func(t *testing.T) {
		acc := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeOAuth,
			Extra:    map[string]any{},
		}
		if acc.IsTLSFingerprintEnabled() {
			t.Fatal("expected tls fingerprint to be disabled")
		}
	})

	t.Run("enabled for setup-token only when switch is on", func(t *testing.T) {
		acc := &Account{
			Platform: PlatformAnthropic,
			Type:     AccountTypeSetupToken,
			Extra: map[string]any{
				"enable_tls_fingerprint": true,
			},
		}
		if !acc.IsTLSFingerprintEnabled() {
			t.Fatal("expected tls fingerprint to be enabled")
		}
	})

	t.Run("disabled for non-anthropic oauth even when switch is on", func(t *testing.T) {
		acc := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeOAuth,
			Extra: map[string]any{
				"enable_tls_fingerprint": true,
			},
		}
		if acc.IsTLSFingerprintEnabled() {
			t.Fatal("expected tls fingerprint to be disabled")
		}
	})
}
