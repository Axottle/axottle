package auth

import (
	"strings"
	"testing"
	"time"
)

func TestTOTPProvisioningAndValidation(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	if len(secret) != 32 {
		t.Fatalf("secret length = %d, want 32", len(secret))
	}
	uri := TOTPProvisioningURI(secret, "operator@example.com", "Axottle")
	for _, want := range []string{"otpauth://totp/", "secret=" + secret, "issuer=Axottle", "digits=6", "period=30"} {
		if !strings.Contains(uri, want) {
			t.Fatalf("provisioning URI %q does not contain %q", uri, want)
		}
	}

	now := time.Unix(1_700_000_000, 0)
	counter := uint64(now.Unix() / 30)
	code, err := totpAtCounter(secret, counter)
	if err != nil {
		t.Fatalf("totpAtCounter: %v", err)
	}
	matched, ok := validateTOTPCounter(secret, code, now)
	if !ok || matched != int64(counter) {
		t.Fatalf("validateTOTPCounter = (%d, %v), want (%d, true)", matched, ok, counter)
	}
	if _, ok := validateTOTPCounter(secret, "00000x", now); ok {
		t.Fatal("non-numeric code unexpectedly validated")
	}
}

func TestTOTPAllowsOnePeriodClockSkew(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	previous := uint64(now.Unix()/30 - 1)
	code, err := totpAtCounter(secret, previous)
	if err != nil {
		t.Fatal(err)
	}
	matched, ok := validateTOTPCounter(secret, code, now)
	if !ok || matched != int64(previous) {
		t.Fatalf("previous-window code rejected: (%d, %v)", matched, ok)
	}
}
