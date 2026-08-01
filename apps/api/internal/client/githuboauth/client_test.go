package githuboauth

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/tensho1026/github-issue-search/apps/api/internal/domain/auth"
)

func TestAuthorizationURLUsesPKCEAndMinimumScope(t *testing.T) {
	client := newTestClient(t, "https://github.example")
	state := auth.NewSecret(strings.Repeat("A", 43))
	location := client.AuthorizationURL(state, strings.Repeat("B", 43))
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	if query.Get("client_id") != "client-id" ||
		query.Get("redirect_uri") !=
			"https://api.issuescout.example/api/auth/github/callback" ||
		query.Get("scope") != "read:user" ||
		query.Get("state") != state.Value() ||
		query.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization query = %v", query)
	}
}

func TestExchangeAndFetchIdentityKeepTokenServerSide(t *testing.T) {
	var token auth.Secret
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		switch request.URL.Path {
		case "/login/oauth/access_token":
			if err := request.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if request.Form.Get("client_secret") != "client-secret-value" ||
				request.Form.Get("code") != "authorization-code" ||
				request.Form.Get("code_verifier") != "pkce-verifier" {
				t.Fatalf("exchange form keys were not preserved")
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(
				writer,
				`{"access_token":"upstream-token","token_type":"bearer","scope":"read:user"}`,
			)
		case "/user":
			if request.Header.Get("Authorization") != "Bearer upstream-token" ||
				request.Header.Get("X-GitHub-Api-Version") != apiVersion {
				t.Fatal("identity request did not use the server-side token")
			}
			writer.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(writer, `{
				"id":583231,
				"login":"octocat",
				"avatar_url":"https://avatars.githubusercontent.com/u/583231",
				"html_url":"https://github.com/octocat"
			}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := newLocalTestClient(t, server.URL)

	var err error
	token, err = client.Exchange(
		context.Background(),
		auth.NewSecret("authorization-code"),
		auth.NewSecret("pkce-verifier"),
	)
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if token.String() != "<redacted>" {
		t.Fatalf("formatted token = %q", token)
	}
	identity, err := client.FetchIdentity(context.Background(), token)
	if err != nil {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
	if identity.UserID != 583231 || identity.Login != "octocat" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestExchangeRejectsExpandedScopesWithoutLeakingCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		writer http.ResponseWriter,
		_ *http.Request,
	) {
		_, _ = io.WriteString(
			writer,
			`{"access_token":"token","token_type":"bearer","scope":"repo,read:user"}`,
		)
	}))
	defer server.Close()
	client := newLocalTestClient(t, server.URL)
	const code = "sensitive-authorization-code"

	_, err := client.Exchange(
		context.Background(),
		auth.NewSecret(code),
		auth.NewSecret("pkce-verifier"),
	)
	if !errors.Is(err, auth.ErrOAuthRejected) {
		t.Fatalf("Exchange() error = %v", err)
	}
	if strings.Contains(err.Error(), code) {
		t.Fatal("Exchange() error exposed the authorization code")
	}
}

func TestFetchIdentityMapsCancellationSafely(t *testing.T) {
	client := newTestClient(t, "https://github.example")
	client.httpClient = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.Canceled
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.FetchIdentity(ctx, auth.NewSecret("token"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("FetchIdentity() error = %v", err)
	}
}

func newTestClient(t *testing.T, origin string) *Client {
	t.Helper()
	authorizeURL := mustURL(t, origin+"/login/oauth/authorize")
	tokenURL := mustURL(t, origin+"/login/oauth/access_token")
	apiURL := mustURL(t, origin)
	callbackURL := mustURL(
		t,
		"https://api.issuescout.example/api/auth/github/callback",
	)
	client, err := NewClient(
		authorizeURL,
		tokenURL,
		apiURL,
		callbackURL,
		"client-id",
		"client-secret-value",
		5*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func newLocalTestClient(t *testing.T, origin string) *Client {
	t.Helper()
	return newTestClient(t, origin)
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) Do(
	request *http.Request,
) (*http.Response, error) {
	return function(request)
}
