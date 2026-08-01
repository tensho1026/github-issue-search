package authhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

func TestSecurePolicyWritesHostOnlyHttpOnlyCookies(t *testing.T) {
	now := time.Date(2026, time.August, 1, 5, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	policy := NewPolicy(true)
	recorder := httptest.NewRecorder()
	policy.WriteSession(
		recorder,
		credential("A"),
		credential("B"),
		expiresAt,
		now,
	)

	headers := recorder.Header().Values("Set-Cookie")
	if len(headers) != 2 {
		t.Fatalf("Set-Cookie count = %d", len(headers))
	}
	for _, header := range headers {
		for _, attribute := range []string{
			"__Host-issuescout_",
			"Path=/",
			"HttpOnly",
			"Secure",
			"SameSite=Lax",
			"Max-Age=3600",
		} {
			if !strings.Contains(header, attribute) {
				t.Errorf("Set-Cookie missing %q: %s", attribute, header)
			}
		}
		if strings.Contains(header, "Domain=") {
			t.Errorf("Set-Cookie contains Domain: %s", header)
		}
	}
}

func TestCredentialsRejectPartialOrMalformedCookies(t *testing.T) {
	policy := NewPolicy(false)
	tests := []struct {
		name    string
		cookies []*http.Cookie
		want    bool
	}{
		{name: "missing"},
		{
			name: "partial",
			cookies: []*http.Cookie{
				requestCookie(
					policy.Names().Session,
					credential("A").Value(),
				),
			},
		},
		{
			name: "malformed",
			cookies: []*http.Cookie{
				requestCookie(policy.Names().Session, "short"),
				requestCookie(
					policy.Names().CSRF,
					credential("B").Value(),
				),
			},
		},
		{
			name: "valid",
			cookies: []*http.Cookie{
				requestCookie(
					policy.Names().Session,
					credential("A").Value(),
				),
				requestCookie(
					policy.Names().CSRF,
					credential("B").Value(),
				),
			},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			for _, cookie := range test.cookies {
				request.AddCookie(cookie)
			}
			_, _, ok := policy.Credentials(request)
			if ok != test.want {
				t.Fatalf("Credentials() ok = %t, want %t", ok, test.want)
			}
		})
	}
}

func TestPolicyClearsAllAuthenticationCookies(t *testing.T) {
	policy := NewPolicy(false)
	recorder := httptest.NewRecorder()
	policy.ClearFlow(recorder)
	policy.ClearSession(recorder)

	headers := recorder.Header().Values("Set-Cookie")
	if len(headers) != 3 {
		t.Fatalf("Set-Cookie count = %d", len(headers))
	}
	for _, header := range headers {
		if !strings.Contains(header, "Max-Age=0") ||
			!strings.Contains(header, "Expires=Thu, 01 Jan 1970") {
			t.Errorf("cookie was not expired: %s", header)
		}
	}
}

func credential(fill string) auth.Secret {
	return auth.NewSecret(strings.Repeat(fill, 43))
}

func requestCookie(name, value string) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}
