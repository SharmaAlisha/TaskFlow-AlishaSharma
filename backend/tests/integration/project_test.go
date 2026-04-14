package integration

import (
	"fmt"
	"testing"

	"github.com/taskflow/backend/tests/testutil"
)

func TestProjectCRUD(t *testing.T) {
	ta := testutil.SetupTestApp(t)
	token, _ := testutil.RegisterAndLogin(t, ta.App)

	var projectID string

	t.Run("create project", func(t *testing.T) {
		body := map[string]string{
			"name":        "Test Project",
			"description": "A project for testing",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/projects/", body, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 201 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(resp)
		projectID = data["id"].(string)
		if projectID == "" {
			t.Fatal("expected project id")
		}
		if data["name"] != "Test Project" {
			t.Fatalf("expected name 'Test Project', got %s", data["name"])
		}
	})

	t.Run("list projects returns created project", func(t *testing.T) {
		resp, err := testutil.DoRequest(ta.App, "GET", "/api/v1/projects/", nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		data, _ := testutil.ParseJSON(resp)
		items := data["data"].([]interface{})
		if len(items) == 0 {
			t.Fatal("expected at least 1 project")
		}

		pagination := data["pagination"].(map[string]interface{})
		if pagination["total"].(float64) < 1 {
			t.Fatal("expected total >= 1")
		}
	})

	t.Run("get project by ID", func(t *testing.T) {
		resp, err := testutil.DoRequest(ta.App, "GET", fmt.Sprintf("/api/v1/projects/%s", projectID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		data, _ := testutil.ParseJSON(resp)
		if data["id"] != projectID {
			t.Fatalf("expected project id %s, got %s", projectID, data["id"])
		}
	})

	t.Run("update project (owner only)", func(t *testing.T) {
		body := map[string]string{
			"name": "Updated Project Name",
		}
		resp, err := testutil.DoRequest(ta.App, "PATCH", fmt.Sprintf("/api/v1/projects/%s", projectID), body, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(resp)
		if data["name"] != "Updated Project Name" {
			t.Fatalf("expected updated name, got %s", data["name"])
		}
	})

	t.Run("update project by non-owner returns 403", func(t *testing.T) {
		otherToken, _ := testutil.RegisterAndLogin(t, ta.App)
		body := map[string]string{"name": "Hacked"}
		resp, err := testutil.DoRequest(ta.App, "PATCH", fmt.Sprintf("/api/v1/projects/%s", projectID), body, otherToken)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("get project stats", func(t *testing.T) {
		resp, err := testutil.DoRequest(ta.App, "GET", fmt.Sprintf("/api/v1/projects/%s/stats", projectID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		data, _ := testutil.ParseJSON(resp)
		if data["by_status"] == nil {
			t.Fatal("expected by_status in stats response")
		}
	})

	t.Run("delete project cascades tasks", func(t *testing.T) {
		taskBody := map[string]string{
			"title":    "Task to be cascaded",
			"priority": "high",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), taskBody, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 201 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 201 for task creation, got %d: %v", resp.StatusCode, data)
		}

		delResp, err := testutil.DoRequest(ta.App, "DELETE", fmt.Sprintf("/api/v1/projects/%s", projectID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if delResp.StatusCode != 204 {
			t.Fatalf("expected 204, got %d", delResp.StatusCode)
		}

		getResp, err := testutil.DoRequest(ta.App, "GET", fmt.Sprintf("/api/v1/projects/%s", projectID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if getResp.StatusCode != 404 {
			t.Fatalf("expected 404 after delete, got %d", getResp.StatusCode)
		}
	})
}
