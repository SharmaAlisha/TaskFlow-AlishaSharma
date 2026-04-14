package integration

import (
	"fmt"
	"testing"

	"github.com/taskflow/backend/tests/testutil"
)

func TestTaskLifecycle(t *testing.T) {
	ta := testutil.SetupTestApp(t)
	token, userID := testutil.RegisterAndLogin(t, ta.App)

	// Create a project first
	projBody := map[string]string{"name": "Task Test Project"}
	projResp, err := testutil.DoRequest(ta.App, "POST", "/api/v1/projects/", projBody, token)
	if err != nil {
		t.Fatal(err)
	}
	projData, _ := testutil.ParseJSON(projResp)
	projectID := projData["id"].(string)

	var taskID string

	t.Run("create task sets creator_id", func(t *testing.T) {
		body := map[string]interface{}{
			"title":       "My First Task",
			"description": "Testing task creation",
			"priority":    "high",
			"assignee_id": userID,
			"due_date":    "2026-05-01",
		}
		resp, err := testutil.DoRequest(ta.App, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), body, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 201 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 201, got %d: %v", resp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(resp)
		taskID = data["id"].(string)
		if data["creator_id"] != userID {
			t.Fatalf("expected creator_id %s, got %s", userID, data["creator_id"])
		}
		if data["status"] != "todo" {
			t.Fatalf("expected default status 'todo', got %s", data["status"])
		}
		if data["version"].(float64) != 1 {
			t.Fatalf("expected version 1, got %v", data["version"])
		}
	})

	t.Run("update task with correct version", func(t *testing.T) {
		body := map[string]interface{}{
			"status":  "in_progress",
			"version": 1,
		}
		resp, err := testutil.DoRequest(ta.App, "PATCH", fmt.Sprintf("/api/v1/tasks/%s", taskID), body, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			data, _ := testutil.ParseJSON(resp)
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}

		data, _ := testutil.ParseJSON(resp)
		if data["status"] != "in_progress" {
			t.Fatalf("expected status in_progress, got %s", data["status"])
		}
		if data["version"].(float64) != 2 {
			t.Fatalf("expected version 2, got %v", data["version"])
		}
	})

	t.Run("update task with stale version returns 409", func(t *testing.T) {
		body := map[string]interface{}{
			"status":  "done",
			"version": 1, // stale
		}
		resp, err := testutil.DoRequest(ta.App, "PATCH", fmt.Sprintf("/api/v1/tasks/%s", taskID), body, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 409 {
			t.Fatalf("expected 409 for stale version, got %d", resp.StatusCode)
		}
	})

	t.Run("list tasks with filters", func(t *testing.T) {
		// Create a second task with different status
		body2 := map[string]interface{}{
			"title":    "Second Task",
			"priority": "low",
		}
		testutil.DoRequest(ta.App, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), body2, token)

		resp, err := testutil.DoRequest(ta.App, "GET",
			fmt.Sprintf("/api/v1/projects/%s/tasks?status=in_progress", projectID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		data, _ := testutil.ParseJSON(resp)
		items := data["data"].([]interface{})
		for _, item := range items {
			task := item.(map[string]interface{})
			if task["status"] != "in_progress" {
				t.Fatalf("filter failed: expected in_progress, got %s", task["status"])
			}
		}

		// Filter by assignee
		resp2, _ := testutil.DoRequest(ta.App, "GET",
			fmt.Sprintf("/api/v1/projects/%s/tasks?assignee=%s", projectID, userID), nil, token)
		if resp2.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp2.StatusCode)
		}
	})

	t.Run("delete task by non-creator non-owner returns 403", func(t *testing.T) {
		otherToken, _ := testutil.RegisterAndLogin(t, ta.App)
		resp, err := testutil.DoRequest(ta.App, "DELETE", fmt.Sprintf("/api/v1/tasks/%s", taskID), nil, otherToken)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 403 {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("delete task by creator succeeds", func(t *testing.T) {
		resp, err := testutil.DoRequest(ta.App, "DELETE", fmt.Sprintf("/api/v1/tasks/%s", taskID), nil, token)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 204 {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}

		// Confirm 404
		getResp, _ := testutil.DoRequest(ta.App, "PATCH", fmt.Sprintf("/api/v1/tasks/%s", taskID),
			map[string]interface{}{"version": 1, "title": "nope"}, token)
		if getResp.StatusCode != 404 {
			t.Fatalf("expected 404 after delete, got %d", getResp.StatusCode)
		}
	})
}
