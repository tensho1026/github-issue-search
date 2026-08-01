// Package authcrypto provides bounded credential generation and authenticated
// encryption for the short-lived OAuth flow cookie.
package authcrypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

const (
	credentialBytes    = 32
	flowCookieVersion  = "v1"
	maximumCookieBytes = 4096
)

var (
	// ErrInvalidConfiguration reports an invalid encryption key or random
	// source without including either value.
	ErrInvalidConfiguration = errors.New("invalid authentication cryptography configuration")
	// ErrInvalidFlowCookie safely covers malformed, modified, and expired
	// OAuth flow cookies.
	ErrInvalidFlowCookie = errors.New("invalid OAuth flow cookie")
)

// Generator creates cryptographically random UUIDs and 256-bit opaque
// credentials. The reader is injectable for deterministic tests.
type Generator struct {
	random io.Reader
}

// NewGenerator constructs a generator backed by crypto/rand.
func NewGenerator() Generator {
	return Generator{random: rand.Reader}
}

// NewGeneratorWithReader constructs a generator with an explicit entropy
// source for deterministic tests.
func NewGeneratorWithReader(random io.Reader) (Generator, error) {
	if random == nil {
		return Generator{}, ErrInvalidConfiguration
	}
	return Generator{random: random}, nil
}

// Opaque creates a 256-bit base64url credential without padding.
func (generator Generator) Opaque() (auth.Secret, error) {
	value := make([]byte, credentialBytes)
	if _, err := io.ReadFull(generator.random, value); err != nil {
		return auth.Secret{}, fmt.Errorf("generate opaque credential: %w", err)
	}
	return auth.NewSecret(base64.RawURLEncoding.EncodeToString(value)), nil
}

// UUID creates a canonical RFC 4122 version 4 identifier.
func (generator Generator) UUID() (string, error) {
	value := make([]byte, 16)
	if _, err := io.ReadFull(generator.random, value); err != nil {
		return "", fmt.Errorf("generate UUID: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value)
	return fmt.Sprintf(
		"%s-%s-%s-%s-%s",
		encoded[0:8],
		encoded[8:12],
		encoded[12:16],
		encoded[16:20],
		encoded[20:32],
	), nil
}

// PKCEChallenge creates the RFC 7636 S256 challenge for a verifier.
func PKCEChallenge(verifier auth.Secret) (string, error) {
	challenge, err := auth.PKCEChallenge(verifier)
	if err != nil {
		return "", ErrInvalidConfiguration
	}
	return challenge, nil
}

// FlowPayload is the encrypted browser binding for one OAuth transaction.
// Its fields must never be logged.
type FlowPayload struct {
	State      auth.Secret
	Verifier   auth.Secret
	ReturnPath string
	ExpiresAt  time.Time
}

type encodedFlowPayload struct {
	State      string `json:"state"`
	Verifier   string `json:"verifier"`
	ReturnPath string `json:"returnPath"`
	ExpiresAt  int64  `json:"expiresAt"`
}

// FlowCodec seals PKCE material in an AES-256-GCM cookie. The database keeps
// only the state hash, so neither store can complete the flow alone.
type FlowCodec struct {
	aead   cipher.AEAD
	random io.Reader
}

// NewFlowCodec constructs a production codec from a 64-character lower-case
// hexadecimal key.
func NewFlowCodec(hexKey string) (*FlowCodec, error) {
	return NewFlowCodecWithReader(hexKey, rand.Reader)
}

// NewFlowCodecWithReader allows deterministic nonce generation in tests.
func NewFlowCodecWithReader(
	hexKey string,
	random io.Reader,
) (*FlowCodec, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 || hexKey != strings.ToLower(hexKey) ||
		random == nil {
		return nil, ErrInvalidConfiguration
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidConfiguration
	}
	return &FlowCodec{aead: aead, random: random}, nil
}

// Seal validates and encrypts a short-lived OAuth flow payload.
func (codec *FlowCodec) Seal(payload FlowPayload) (string, error) {
	if codec == nil || codec.aead == nil ||
		!validCredential(payload.State.Value()) ||
		!validCredential(payload.Verifier.Value()) ||
		payload.ExpiresAt.IsZero() {
		return "", ErrInvalidFlowCookie
	}
	returnPath, err := auth.ValidateReturnPath(payload.ReturnPath)
	if err != nil {
		return "", ErrInvalidFlowCookie
	}
	plaintext, err := json.Marshal(encodedFlowPayload{
		State:      payload.State.Value(),
		Verifier:   payload.Verifier.Value(),
		ReturnPath: returnPath,
		ExpiresAt:  payload.ExpiresAt.UTC().Unix(),
	})
	if err != nil {
		return "", ErrInvalidFlowCookie
	}
	nonce := make([]byte, codec.aead.NonceSize())
	if _, err := io.ReadFull(codec.random, nonce); err != nil {
		return "", fmt.Errorf("generate OAuth flow nonce: %w", err)
	}
	sealed := codec.aead.Seal(nil, nonce, plaintext, []byte(flowCookieVersion))
	value := flowCookieVersion + "." +
		base64.RawURLEncoding.EncodeToString(append(nonce, sealed...))
	if len(value) > maximumCookieBytes {
		return "", ErrInvalidFlowCookie
	}
	return value, nil
}

// Open authenticates, decrypts, and expires an OAuth flow payload.
func (codec *FlowCodec) Open(
	value string,
	now time.Time,
) (FlowPayload, error) {
	if codec == nil || codec.aead == nil ||
		len(value) == 0 || len(value) > maximumCookieBytes {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	version, encoded, found := strings.Cut(value, ".")
	if !found || version != flowCookieVersion {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	sealed, base64Err := base64.RawURLEncoding.DecodeString(encoded)
	if base64Err != nil || len(sealed) <= codec.aead.NonceSize() {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	nonce := sealed[:codec.aead.NonceSize()]
	ciphertext := sealed[codec.aead.NonceSize():]
	plaintext, decryptErr := codec.aead.Open(
		nil,
		nonce,
		ciphertext,
		[]byte(flowCookieVersion),
	)
	if decryptErr != nil {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	decoder := json.NewDecoder(bytes.NewReader(plaintext))
	decoder.DisallowUnknownFields()
	var encodedPayload encodedFlowPayload
	if err := decoder.Decode(&encodedPayload); err != nil {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	expiresAt := time.Unix(encodedPayload.ExpiresAt, 0).UTC()
	if !expiresAt.After(now.UTC()) ||
		!validCredential(encodedPayload.State) ||
		!validCredential(encodedPayload.Verifier) {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	returnPath, pathErr := auth.ValidateReturnPath(encodedPayload.ReturnPath)
	if pathErr != nil {
		return FlowPayload{}, ErrInvalidFlowCookie
	}
	return FlowPayload{
		State:      auth.NewSecret(encodedPayload.State),
		Verifier:   auth.NewSecret(encodedPayload.Verifier),
		ReturnPath: returnPath,
		ExpiresAt:  expiresAt,
	}, nil
}

func validCredential(value string) bool {
	return auth.IsOpaqueCredential(value)
}
