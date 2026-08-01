package auth

import (
	"fmt"
	"testing"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/user"
)

func TestValidateReturnPath(t *testing.T) {
	tests := map[string]struct {
		raw  string
		want string
		ok   bool
	}{
		"default":          {raw: "", want: "/", ok: true},
		"path and query":   {raw: "/workspace?tab=saved", want: "/workspace?tab=saved", ok: true},
		"network path":     {raw: "//attacker.example", ok: false},
		"absolute URL":     {raw: "https://attacker.example", ok: false},
		"backslash escape": {raw: `/\attacker.example`, ok: false},
		"fragment":         {raw: "/workspace#secret", ok: false},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := ValidateReturnPath(test.raw)
			if (err == nil) != test.ok || got != test.want {
				t.Fatalf("ValidateReturnPath(%q) = %q, %v", test.raw, got, err)
			}
		})
	}
}

func TestDigestAndSecretProtectCredentials(t *testing.T) {
	//nolint:gosec // This synthetic value verifies redaction and is not a secret.
	const credential = "credential-that-must-not-format"
	digest := Hash(credential)
	if !digest.Matches(credential) || digest.Matches("different") {
		t.Fatal("digest comparison failed")
	}
	secret := NewSecret(credential)
	if got := fmt.Sprintf("%s %v %#v", secret, secret, secret); got !=
		"<redacted> <redacted> <redacted>" {
		t.Fatalf("formatted secret = %q", got)
	}
}

func TestGitHubIdentityValidation(t *testing.T) {
	login, err := user.ParseUsername("octocat")
	if err != nil {
		t.Fatal(err)
	}
	identity := GitHubIdentity{
		UserID:     1,
		Login:      login,
		AvatarURL:  "https://avatars.githubusercontent.com/u/1",
		ProfileURL: "https://github.com/octocat",
	}
	if err := identity.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	identity.ProfileURL = "javascript:alert(1)"
	if err := identity.Validate(); err == nil {
		t.Fatal("Validate() accepted unsafe profile URL")
	}
}
