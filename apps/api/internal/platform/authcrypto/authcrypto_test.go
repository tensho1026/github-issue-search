package authcrypto

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

func TestGeneratorCreatesBoundedCredentialsAndUUIDs(t *testing.T) {
	random := bytes.NewReader(bytes.Repeat([]byte{0x7f}, 48))
	generator, err := NewGeneratorWithReader(random)
	if err != nil {
		t.Fatal(err)
	}
	credential, err := generator.Opaque()
	if err != nil {
		t.Fatal(err)
	}
	if len(credential.Value()) != 43 {
		t.Fatalf("credential length = %d", len(credential.Value()))
	}
	id, err := generator.UUID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "7f7f7f7f-7f7f-4f7f-bf7f-7f7f7f7f7f7f" {
		t.Fatalf("UUID() = %q", id)
	}
}

func TestFlowCodecRoundTripRejectsTamperingAndExpiry(t *testing.T) {
	now := time.Date(2026, time.August, 1, 4, 0, 0, 0, time.UTC)
	nonce := bytes.Repeat([]byte{0x42}, 12)
	codec, err := NewFlowCodecWithReader(
		strings.Repeat("ab", 32),
		bytes.NewReader(nonce),
	)
	if err != nil {
		t.Fatal(err)
	}
	state := auth.NewSecret(strings.Repeat("A", 43))
	verifier := auth.NewSecret(strings.Repeat("B", 43))
	sealed, err := codec.Seal(FlowPayload{
		State:      state,
		Verifier:   verifier,
		ReturnPath: "/workspace?tab=saved",
		ExpiresAt:  now.Add(10 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Seal() error = %v", err)
	}
	payload, err := codec.Open(sealed, now)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if payload.State.Value() != state.Value() ||
		payload.Verifier.Value() != verifier.Value() ||
		payload.ReturnPath != "/workspace?tab=saved" {
		t.Fatal("Open() did not preserve the payload")
	}
	tampered := sealed[:len(sealed)-1] + "A"
	if _, err := codec.Open(tampered, now); !errors.Is(err, ErrInvalidFlowCookie) {
		t.Fatalf("tampered Open() error = %v", err)
	}
	if _, err := codec.Open(sealed, now.Add(10*time.Minute)); !errors.Is(
		err,
		ErrInvalidFlowCookie,
	) {
		t.Fatalf("expired Open() error = %v", err)
	}
}

func TestPKCEChallengeRejectsNonRandomVerifier(t *testing.T) {
	if _, err := PKCEChallenge(auth.NewSecret("short")); !errors.Is(
		err,
		ErrInvalidConfiguration,
	) {
		t.Fatalf("PKCEChallenge() error = %v", err)
	}
}
