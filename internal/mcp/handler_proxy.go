package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/pstrr/kronos/internal/apiclient"
)

// ProxyHandler implements MCP tool handlers by proxying requests to the
// Kronos REST API via an authenticated HTTP client. It is used by the stdio
// server when running in proxy mode (connecting to an existing daemon).
type ProxyHandler struct {
	client *apiclient.Client
}

// NewProxyHandler creates a ProxyHandler with the given API client.
func NewProxyHandler(client *apiclient.Client) *ProxyHandler {
	return &ProxyHandler{client: client}
}

// --- helpers ---

// apiResult calls the API client, checks the response, extracts data, and
// returns it as a JSON tool result. On API error it returns a tool error.
func (h *ProxyHandler) apiResult(resp map[string]interface{}, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API request failed: %v", err)), nil
	}
	data, err := apiclient.ExtractData(resp)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return jsonResult(data)
}

func taskIDParam(req mcp.CallToolRequest) (string, error) {
	id := mcp.ParseInt64(req, "task_id", 0)
	if id == 0 {
		return "", fmt.Errorf("task_id is required")
	}
	return strconv.FormatInt(id, 10), nil
}

// --- handlers ---

func (h *ProxyHandler) handleListTasks(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path := "/api/v1/tasks?size=100"
	if t := mcp.ParseString(req, "type", ""); t != "" {
		path += "&type=" + t
	}
	if _, ok := req.GetArguments()["enabled"]; ok {
		enabled := mcp.ParseBoolean(req, "enabled", true)
		path += "&enabled=" + strconv.FormatBool(enabled)
	}
	resp, err := h.client.Get(path)
	return h.apiResultUnwrapPaged(resp, err)
}

// apiResultUnwrapPaged extracts paged data and returns items+total as tool result.
func (h *ProxyHandler) apiResultUnwrapPaged(resp map[string]interface{}, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API request failed: %v", err)), nil
	}
	data, extractErr := apiclient.ExtractData(resp)
	if extractErr != nil {
		return mcp.NewToolResultError(extractErr.Error()), nil
	}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return jsonResult(data)
	}
	return jsonResult(map[string]any{
		"total": dataMap["total"],
		"tasks": dataMap["items"],
	})
}

func (h *ProxyHandler) handleCreateTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	body := buildTaskBody(req, "name", "type", "schedule_type", "schedule_expr", "target", "timeout", "retry_count", "enabled")
	jsonBody, _ := json.Marshal(body)
	resp, err := h.client.Post("/api/v1/tasks", jsonBody)
	return h.apiResult(resp, err)
}

func (h *ProxyHandler) handleUpdateTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	body := buildTaskBody(req, "name", "type", "schedule_type", "schedule_expr", "target", "timeout", "retry_count", "enabled")
	jsonBody, _ := json.Marshal(body)
	resp, apiErr := h.client.Put("/api/v1/tasks/"+id, jsonBody)
	return h.apiResult(resp, apiErr)
}

func (h *ProxyHandler) handleDeleteTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, apiErr := h.client.Delete("/api/v1/tasks/" + id)
	if apiErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API request failed: %v", apiErr)), nil
	}
	if checkErr := apiclient.CheckResponse(resp); checkErr != nil {
		return mcp.NewToolResultError(checkErr.Error()), nil
	}
	return jsonResult(map[string]any{
		"message": fmt.Sprintf("task %s deleted", id),
		"task_id": id,
	})
}

func (h *ProxyHandler) handleRunTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, apiErr := h.client.Post("/api/v1/tasks/"+id+"/run", nil)
	return h.apiResult(resp, apiErr)
}

func (h *ProxyHandler) handleGetTaskStatus(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, apiErr := h.client.Get("/api/v1/tasks/" + id)
	return h.apiResult(resp, apiErr)
}

func (h *ProxyHandler) handleGetTaskHistory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	limit := mcp.ParseInt(req, "limit", 10)
	if limit <= 0 {
		limit = 10
	}
	path := fmt.Sprintf("/api/v1/tasks/%s/runs?size=%d", id, limit)
	resp, apiErr := h.client.Get(path)
	if apiErr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API request failed: %v", apiErr)), nil
	}
	data, extractErr := apiclient.ExtractData(resp)
	if extractErr != nil {
		return mcp.NewToolResultError(extractErr.Error()), nil
	}
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return jsonResult(data)
	}
	return jsonResult(map[string]any{
		"task_id": id,
		"total":   dataMap["total"],
		"runs":    dataMap["items"],
	})
}

func (h *ProxyHandler) handleEnableTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, apiErr := h.client.Post("/api/v1/tasks/"+id+"/enable", nil)
	return h.apiResult(resp, apiErr)
}

func (h *ProxyHandler) handleDisableTask(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, err := taskIDParam(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	resp, apiErr := h.client.Post("/api/v1/tasks/"+id+"/disable", nil)
	return h.apiResult(resp, apiErr)
}

func (h *ProxyHandler) handleServerStatus(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Get server status
	statusResp, err := h.client.Get("/api/v1/status")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("API request failed: %v", err)), nil
	}
	statusData, extractErr := apiclient.ExtractData(statusResp)
	if extractErr != nil {
		return mcp.NewToolResultError(extractErr.Error()), nil
	}

	result, ok := statusData.(map[string]interface{})
	if !ok {
		result = map[string]interface{}{}
	}

	// Get total task count
	tasksResp, err := h.client.Get("/api/v1/tasks?size=1")
	if err == nil {
		if tasksData, extractErr := apiclient.ExtractData(tasksResp); extractErr == nil {
			if td, ok := tasksData.(map[string]interface{}); ok {
				result["total_tasks"] = td["total"]
			}
		}
	}

	result["mode"] = "proxy"

	return jsonResult(result)
}

// --- body builder ---

// buildTaskBody extracts only the provided fields from the MCP request
// arguments and returns them as a map suitable for JSON marshalling.
func buildTaskBody(req mcp.CallToolRequest, fields ...string) map[string]interface{} {
	args := req.GetArguments()
	body := make(map[string]interface{})
	for _, f := range fields {
		if v, ok := args[f]; ok {
			body[f] = v
		}
	}
	return body
}
