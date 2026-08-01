// Package authhttp defines the browser-cookie boundary for authentication.
// It keeps cookie names, attributes, expiry, and credential parsing consistent
// across handlers and middleware.
package authhttp

import (
	"net/http"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

const (
	sessionCookieName       = "issuescout_session"
	csrfCookieName          = "issuescout_csrf"
	flowCookieName          = "issuescout_oauth_flow"
	secureSessionCookieName = "__Host-issuescout_session"
	secureCSRFCookieName    = "__Host-issuescout_csrf"
	secureFlowCookieName    = "__Host-issuescout_oauth_flow"
	maximumFlowCookieBytes  = 4096
)

// Names contains the exact cookie identifiers used by middleware.
type Names struct {
	Session string
	CSRF    string
	Flow    string
}

// Policy owns all authentication cookie attributes. A zero Policy is invalid;
// callers construct one explicitly so local HTTP and deployed HTTPS behavior
// cannot be confused.
type Policy struct {
	names  Names
	secure bool
}

// NewPolicy constructs a local or deployed cookie policy. Secure policies use
// the __Host- prefix, which prevents Domain attributes and requires Path=/.
func NewPolicy(secure bool) Policy {
	if secure {
		return Policy{
			names: Names{
				Session: secureSessionCookieName,
				CSRF:    secureCSRFCookieName,
				Flow:    secureFlowCookieName,
			},
			secure: true,
		}
	}
	return Policy{
		names: Names{
			Session: sessionCookieName,
			CSRF:    csrfCookieName,
			Flow:    flowCookieName,
		},
	}
}

// Names returns a copy of the configured cookie identifiers.
func (policy Policy) Names() Names {
	return policy.names
}

// Credentials returns validated session and CSRF cookies. Invalid or partial
// credentials are treated as anonymous before any persistence call occurs.
func (policy Policy) Credentials(
	request *http.Request,
) (auth.Secret, auth.Secret, bool) {
	if request == nil || !policy.valid() {
		return auth.Secret{}, auth.Secret{}, false
	}
	sessionCookie, sessionErr := request.Cookie(policy.names.Session)
	csrfCookie, csrfErr := request.Cookie(policy.names.CSRF)
	if sessionErr != nil || csrfErr != nil ||
		!auth.IsOpaqueCredential(sessionCookie.Value) ||
		!auth.IsOpaqueCredential(csrfCookie.Value) {
		return auth.Secret{}, auth.Secret{}, false
	}
	return auth.NewSecret(sessionCookie.Value),
		auth.NewSecret(csrfCookie.Value),
		true
}

// Flow returns one bounded encrypted OAuth-flow cookie.
func (policy Policy) Flow(request *http.Request) (string, bool) {
	if request == nil || !policy.valid() {
		return "", false
	}
	cookie, err := request.Cookie(policy.names.Flow)
	if err != nil || cookie.Value == "" ||
		len(cookie.Value) > maximumFlowCookieBytes {
		return "", false
	}
	return cookie.Value, true
}

// WriteFlow stores the encrypted state and PKCE browser binding until the
// authorization callback.
func (policy Policy) WriteFlow(
	writer http.ResponseWriter,
	value string,
	expiresAt time.Time,
	now time.Time,
) {
	if writer == nil || !policy.valid() || value == "" ||
		len(value) > maximumFlowCookieBytes ||
		!expiresAt.After(now) {
		return
	}
	http.SetCookie(writer, policy.cookie(
		policy.names.Flow,
		value,
		expiresAt,
		maxAge(expiresAt, now),
	))
}

// WriteSession stores rotating opaque session and CSRF credentials. Both
// cookies are HttpOnly; the session endpoint returns the CSRF value to the
// frontend for in-memory use.
func (policy Policy) WriteSession(
	writer http.ResponseWriter,
	sessionToken auth.Secret,
	csrfToken auth.Secret,
	expiresAt time.Time,
	now time.Time,
) {
	if writer == nil || !policy.valid() ||
		!auth.IsOpaqueCredential(sessionToken.Value()) ||
		!auth.IsOpaqueCredential(csrfToken.Value()) ||
		!expiresAt.After(now) {
		return
	}
	age := maxAge(expiresAt, now)
	http.SetCookie(writer, policy.cookie(
		policy.names.Session,
		sessionToken.Value(),
		expiresAt,
		age,
	))
	http.SetCookie(writer, policy.cookie(
		policy.names.CSRF,
		csrfToken.Value(),
		expiresAt,
		age,
	))
}

// ClearFlow expires the OAuth-flow browser binding.
func (policy Policy) ClearFlow(writer http.ResponseWriter) {
	policy.clear(writer, policy.names.Flow)
}

// ClearSession expires both browser session credentials.
func (policy Policy) ClearSession(writer http.ResponseWriter) {
	policy.clear(writer, policy.names.Session)
	policy.clear(writer, policy.names.CSRF)
}

func (policy Policy) valid() bool {
	return policy.names.Session != "" &&
		policy.names.CSRF != "" &&
		policy.names.Flow != ""
}

func (policy Policy) cookie(
	name string,
	value string,
	expiresAt time.Time,
	maximumAge int,
) *http.Cookie {
	//nolint:gosec // Secure is required by validated staging/production config;
	// loopback development intentionally supports plain HTTP.
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maximumAge,
		Expires:  expiresAt.UTC(),
		HttpOnly: true,
		Secure:   policy.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func (policy Policy) clear(
	writer http.ResponseWriter,
	name string,
) {
	if writer == nil || !policy.valid() {
		return
	}
	http.SetCookie(writer, policy.cookie(
		name,
		"",
		time.Unix(1, 0).UTC(),
		-1,
	))
}

func maxAge(expiresAt time.Time, now time.Time) int {
	seconds := int(expiresAt.Sub(now).Seconds())
	if seconds < 1 {
		return 1
	}
	return seconds
}
