package integration

import (
	"net/http"
	"testing"

	"github.com/taskflow/backend/tests/testutil"
)

func TestAuthFlow(t *testing.T) {
	ta := testutil.SetupTestApp(t)

	t.Run("register new user", func(t *testing.T) {
		body := map[string]string{
			"name":     "Jane Doe",
			"email":    "jane@example.com",
			"password": "securepass123",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/register", body, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 201 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(resp)
		if data["access_token"] == nil || data["access_token"] == "" {
			t.Fatal("expected access_token in response")
		}
		user := data["user"].(map[string]interface{})
		if user["email"] != "jane@example.com" {
			t.Fatalf("expected email jane@example.com, got %s", user["email"])
		}

		var refreshCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "refresh_token" {
				refreshCookie = c
			}
		}
		if refreshCookie == nil {
			t.Fatal("expected refresh_token cookie")
		}
		if !refreshCookie.HttpOnly {
			t.Fatal("refresh_token cookie should be HttpOnly")
		}
	})

	t.Run("register duplicate email returns 400", func(t *testing.T) {
		body := map[string]string{
			"name":     "Jane Again",
			"email":    "jane@example.com",
			"password": "anotherpass123",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/register", body, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("register validation errors", func(t *testing.T) {
		body := map[string]string{
			"name":     "",
			"email":    "not-an-email",
			"password": "short",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/register", body, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 400 {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		data, _ := testutil.ParseJSON(resp)
		if data["fields"] == nil {
			t.Fatal("expected fields in validation error")
		}
	})

	t.Run("login with correct credentials", func(t *testing.T) {
		body := map[string]string{
			"email":    "jane@example.com",
			"password": "securepass123",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/login", body, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
	})

	t.Run("login with wrong password returns 401", func(t *testing.T) {
		body := map[string]string{
			"email":    "jane@example.com",
			"password": "wrongpass",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/login", body, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("access protected endpoint with token", func(t *testing.T) {
		token, _ := testutil.RegisterAndLogin(t, ta.App)

		resp, err := testutil.DoRequest(ta.App, "GET", "/api/v1/projects/", nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("access protected endpoint without token returns 401", func(t *testing.T) {
		resp, err := testutil.DoRequest(ta.App, "GET", "/api/v1/projects/", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 401 {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("refresh token flow", func(t *testing.T) {
		body := map[string]string{
			"name":     "Refresh User",
			"email":    "refresh@example.com",
			"password": "securepass123",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/auth/register", body, "")
		if err != nil {
			t.Fatal(err)
		}

		var refreshCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == "refresh_token" {
				refreshCookie = c
			}
		}
		if refreshCookie == nil {
			t.Fatal("no refresh_token cookie from register")
		}

		refreshResp, err := testutil.DoRequestWithCookie(ta.App, "POST", "/api/v1/auth/refresh", nil, []*http.Cookie{refreshCookie})
		if err != nil {
			t.Fatal(err)
		}
		if refreshResp.StatusCode != 200 {
			data, _ := testutil.ParseJSON(refreshResp)
			t.Fatalf("expected 200 on refresh, got %d: %v", refreshResp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(refreshResp)
		if data["access_token"] == nil || data["access_token"] == "" {
			t.Fatal("expected new access_token from refresh")
		}
	})
}
