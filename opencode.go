package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OpencodeClient handles communication with OpenCode server
type OpencodeClient struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
	mu         sync.Mutex

	// Sesiones por chat ID
	sessions map[int64]string
}

// PromptRequest body para /session/{id}/message
type PromptRequest struct {
	MessageID *string   `json:"messageID,omitempty"`
	NoReply   bool      `json:"noReply,omitempty"`
	Parts     []Part    `json:"parts"`
	Model     *ModelRef `json:"model,omitempty"`
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type ModelRef struct {
	ProviderID string `json:"providerID"`
	ModelID    string `json:"modelID"`
}

// PromptResponse respuesta del servidor
type PromptResponse struct {
	Info  MessageInfo    `json:"info"`
	Parts []ResponsePart `json:"parts"`
}

type MessageInfo struct {
	MessageID  string `json:"messageID"`
	Role       string `json:"role"`
	ProviderID string `json:"providerID,omitempty"`
	ModelID    string `json:"modelID,omitempty"`
}

type ResponsePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// SessionResponse para crear sesiones
type SessionResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// HealthResponse
type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version"`
}

// NewOpencodeClient creates a new OpenCode client
func NewOpencodeClient(baseURL, username, password string) *OpencodeClient {
	return &OpencodeClient{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 120 * time.Second},
		sessions:   make(map[int64]string),
	}
}

// HealthCheck verifica que el servidor esté corriendo
func (c *OpencodeClient) HealthCheck(ctx context.Context) error {
	url := c.baseURL + "/global/health"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	var health HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return fmt.Errorf("failed to decode health response: %w", err)
	}

	if !health.Healthy {
		return fmt.Errorf("server not healthy")
	}

	log.Printf("✅ OpenCode server healthy (version: %s)", health.Version)
	return nil
}

// GetOrCreateSession obtener o crear sesión para un chat
func (c *OpencodeClient) GetOrCreateSession(ctx context.Context, chatID int64, projectDir string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Retornar sesión existente si hay
	if sessionID, ok := c.sessions[chatID]; ok {
		return sessionID, nil
	}

	// Crear nueva sesión
	sessionID, err := c.createSession(ctx, projectDir)
	if err != nil {
		return "", err
	}

	c.sessions[chatID] = sessionID
	log.Printf("📝 Created new session %s for chat %d", sessionID, chatID)
	return sessionID, nil
}

// createSession POST /session
func (c *OpencodeClient) createSession(ctx context.Context, projectDir string) (string, error) {
	url := c.baseURL + "/session"

	body := map[string]interface{}{
		"title": "Telegram Bridge Session",
	}
	if projectDir != "" && projectDir != "." {
		body["directory"] = projectDir
	}

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create session failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var session SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&session); err != nil {
		return "", err
	}

	return session.ID, nil
}

// SendPrompt envía un prompt a OpenCode y retorna la respuesta
func (c *OpencodeClient) SendPrompt(ctx context.Context, sessionID, text string) (string, error) {
	url := fmt.Sprintf("%s/session/%s/message", c.baseURL, sessionID)

	promptReq := PromptRequest{
		Parts: []Part{
			{Type: "text", Text: text},
		},
		NoReply: false,
	}

	bodyJSON, err := json.Marshal(promptReq)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addAuth(req)

	log.Printf("📤 Sending prompt to session %s: %s", sessionID, truncate(text, 50))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("prompt request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("prompt failed: %d - %s", resp.StatusCode, string(respBody))
	}

	var promptResp PromptResponse
	if err := json.NewDecoder(resp.Body).Decode(&promptResp); err != nil {
		return "", fmt.Errorf("decode response failed: %w", err)
	}

	// Extraer texto de la respuesta
	var responseText string
	for _, part := range promptResp.Parts {
		if part.Type == "text" && part.Text != "" {
			responseText += part.Text + "\n"
		}
	}

	responseText = strings.TrimSpace(responseText)
	if responseText == "" {
		return "", fmt.Errorf("no text response from OpenCode")
	}

	log.Printf("📥 Received response (%d chars)", len(responseText))
	return responseText, nil
}

// ClearSession borra la sesión de un chat
func (c *OpencodeClient) ClearSession(chatID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, chatID)
	log.Printf("🗑️ Cleared session for chat %d", chatID)
}

// AbortSession aborta una sesión en ejecución
func (c *OpencodeClient) AbortSession(ctx context.Context, sessionID string) error {
	url := fmt.Sprintf("%s/session/%s/abort", c.baseURL, sessionID)

	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return err
	}
	c.addAuth(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// addAuth agrega autenticación básica
func (c *OpencodeClient) addAuth(req *http.Request) {
	if c.password != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

// truncate string helper
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
