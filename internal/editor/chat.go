package editor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	openRouterChatURL      = "https://openrouter.ai/api/v1/chat/completions"
	defaultOpenRouterModel = "openai/gpt-4o-mini"
	maxAssistantToolRounds = 8
)

type chatRequest struct {
	Messages    []chatMessage `json:"messages"`
	CurrentFlag string        `json:"currentFlag,omitempty"`
	Model       string        `json:"model,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openRouterChunk struct {
	Choices []openRouterChunkChoice `json:"choices"`
	Usage   *openRouterUsage        `json:"usage,omitempty"`
	Error   *openRouterError        `json:"error,omitempty"`
}

type openRouterUsage struct {
	PromptTokens     int      `json:"prompt_tokens"`
	CompletionTokens int      `json:"completion_tokens"`
	TotalTokens      int      `json:"total_tokens"`
	Cost             *float64 `json:"cost,omitempty"`
}

type openRouterChunkChoice struct {
	Index        int                  `json:"index"`
	Delta        openRouterChunkDelta `json:"delta"`
	FinishReason *string              `json:"finish_reason,omitempty"`
}

type openRouterChunkDelta struct {
	Role      string                    `json:"role,omitempty"`
	Content   string                    `json:"content,omitempty"`
	ToolCalls []openRouterChunkToolCall `json:"tool_calls,omitempty"`
}

type openRouterChunkToolCall struct {
	Index    int                    `json:"index"`
	ID       string                 `json:"id,omitempty"`
	Type     string                 `json:"type,omitempty"`
	Function *chatToolFunctionDelta `json:"function,omitempty"`
}

type chatToolFunctionDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type openRouterError struct {
	Message string `json:"message"`
}

// HandleChat starts a new assistant turn and returns the streamId immediately.
// The actual streaming work runs in a background goroutine that writes events
// into a streamSession, so clients can disconnect (e.g. close the panel or
// navigate to a new page) and re-attach later via /api/chat/stream.
func (h *WebHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := apiKeyFromRequest(r)
	if apiKey == "" {
		http.Error(w, "Connect OpenRouter to use the assistant.", http.StatusUnauthorized)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body.", http.StatusBadRequest)
		return
	}
	if len(req.Messages) == 0 {
		http.Error(w, "At least one message is required.", http.StatusBadRequest)
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = os.Getenv("OPENROUTER_MODEL")
	}
	if model == "" {
		model = defaultOpenRouterModel
	}

	referer := openRouterReferer(r)
	sess := defaultStreamRegistry.Create()

	go h.runChatTurn(sess, apiKey, model, req, referer)

	writeJSON(w, http.StatusOK, map[string]string{"streamId": sess.id})
}

// HandleChatStream replays buffered events for a stream session and tails any
// new events as they arrive. It is safe to attach multiple times: each new
// connection receives the full buffer from the start.
func (h *WebHandler) HandleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apiKey := apiKeyFromRequest(r)
	if apiKey == "" {
		http.Error(w, "Connect OpenRouter to use the assistant.", http.StatusUnauthorized)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing stream id.", http.StatusBadRequest)
		return
	}
	sess := defaultStreamRegistry.Get(id)
	if sess == nil {
		http.Error(w, "Stream not found.", http.StatusNotFound)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	notify, unsub := sess.Subscribe()
	defer unsub()

	ctx := r.Context()
	after := 0
	for {
		events, finished := sess.Snapshot(after)
		for _, evt := range events {
			fmt.Fprintf(w, "data: %s\n\n", evt)
		}
		after += len(events)
		flusher.Flush()
		if finished {
			return
		}
		select {
		case <-notify:
		case <-ctx.Done():
			return
		}
	}
}

// runChatTurn owns the lifetime of a single assistant turn. It runs in its own
// goroutine and is decoupled from the HTTP request that started it, so the
// stream survives client disconnects.
func (h *WebHandler) runChatTurn(sess *streamSession, apiKey, model string, req chatRequest, referer string) {
	defer sess.Finish()

	send := sess.Append
	ctx := context.Background()

	messages := append([]chatMessage{
		{Role: "system", Content: assistantSystemPrompt(req.CurrentFlag)},
	}, req.Messages...)

	var newMessages []chatMessage
	var totalCost float64
	var totalTokens int
	hasCost := false

	for round := 0; round < maxAssistantToolRounds; round++ {
		assistantMsg, roundUsage, err := streamOpenRouter(ctx, apiKey, model, referer, messages, send)
		if err != nil {
			send(map[string]any{"type": "error", "error": err.Error()})
			return
		}
		if roundUsage != nil {
			if roundUsage.Cost != nil {
				totalCost += *roundUsage.Cost
				hasCost = true
			}
			totalTokens += roundUsage.TotalTokens
		}
		messages = append(messages, assistantMsg)
		newMessages = append(newMessages, assistantMsg)

		if len(assistantMsg.ToolCalls) == 0 {
			break
		}

		for _, tc := range assistantMsg.ToolCalls {
			send(map[string]any{
				"type":      "tool_call",
				"id":        tc.ID,
				"name":      tc.Function.Name,
				"arguments": tc.Function.Arguments,
			})
			result, execErr := executeAssistantTool(ctx, h, tc.Function.Name, tc.Function.Arguments)
			toolMsg := chatMessage{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
			}
			if execErr != nil {
				toolMsg.Content = execErr.Error()
				send(map[string]any{
					"type":  "tool_result",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"ok":    false,
					"error": execErr.Error(),
				})
			} else {
				toolMsg.Content = result
				send(map[string]any{
					"type": "tool_result",
					"id":   tc.ID,
					"name": tc.Function.Name,
					"ok":   true,
				})
			}
			messages = append(messages, toolMsg)
			newMessages = append(newMessages, toolMsg)
		}
	}

	donePayload := map[string]any{
		"type":     "done",
		"messages": newMessages,
	}
	if hasCost {
		donePayload["cost"] = totalCost
	}
	if totalTokens > 0 {
		donePayload["tokens"] = totalTokens
	}
	send(donePayload)
}

func streamOpenRouter(
	ctx context.Context,
	apiKey, model, referer string,
	messages []chatMessage,
	send func(map[string]any),
) (chatMessage, *openRouterUsage, error) {
	payload := map[string]any{
		"model":    model,
		"messages": messages,
		"tools":    openRouterTools(),
		"stream":   true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return chatMessage{}, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterChatURL, bytes.NewReader(body))
	if err != nil {
		return chatMessage{}, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("HTTP-Referer", referer)
	req.Header.Set("X-Title", "mongo-openfeature-go Flag Manager")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatMessage{}, nil, fmt.Errorf("openrouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		msg := strings.TrimSpace(string(respBody))
		if msg == "" {
			msg = resp.Status
		}
		return chatMessage{}, nil, fmt.Errorf("openrouter: %s", msg)
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	final := chatMessage{Role: "assistant"}
	toolCallsByIndex := map[int]*chatToolCall{}
	var toolCallOrder []int
	var roundUsage *openRouterUsage

	send(map[string]any{"type": "message_start"})

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}

		var chunk openRouterChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return chatMessage{}, nil, fmt.Errorf("openrouter: %s", chunk.Error.Message)
		}

		if chunk.Usage != nil {
			roundUsage = chunk.Usage
			usagePayload := map[string]any{"type": "usage"}
			if chunk.Usage.Cost != nil {
				usagePayload["cost"] = *chunk.Usage.Cost
			}
			if chunk.Usage.TotalTokens > 0 {
				usagePayload["tokens"] = chunk.Usage.TotalTokens
			}
			if chunk.Usage.PromptTokens > 0 {
				usagePayload["prompt_tokens"] = chunk.Usage.PromptTokens
			}
			if chunk.Usage.CompletionTokens > 0 {
				usagePayload["completion_tokens"] = chunk.Usage.CompletionTokens
			}
			send(usagePayload)
		}

		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta

		if delta.Content != "" {
			final.Content += delta.Content
			send(map[string]any{
				"type":    "delta",
				"content": delta.Content,
			})
		}

		for _, dtc := range delta.ToolCalls {
			tc, exists := toolCallsByIndex[dtc.Index]
			if !exists {
				tc = &chatToolCall{Type: "function"}
				toolCallsByIndex[dtc.Index] = tc
				toolCallOrder = append(toolCallOrder, dtc.Index)
			}
			if dtc.ID != "" {
				tc.ID = dtc.ID
			}
			if dtc.Type != "" {
				tc.Type = dtc.Type
			}
			if dtc.Function != nil {
				if dtc.Function.Name != "" {
					tc.Function.Name = dtc.Function.Name
				}
				if dtc.Function.Arguments != "" {
					tc.Function.Arguments += dtc.Function.Arguments
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return chatMessage{}, nil, fmt.Errorf("reading stream: %w", err)
	}

	for _, idx := range toolCallOrder {
		if tc, ok := toolCallsByIndex[idx]; ok {
			final.ToolCalls = append(final.ToolCalls, *tc)
		}
	}

	send(map[string]any{"type": "message_end"})

	return final, roundUsage, nil
}
