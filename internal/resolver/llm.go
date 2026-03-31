package resolver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/banfen321/omnix/internal/config"
)

type LLMClient struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewLLMClient(cfg *config.Config) *LLMClient {
	return &LLMClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (l *LLMClient) ResolvePackage(pkgName, ecosystem string, candidates []string) (string, error) {
	candidateStr := ""
	if len(candidates) > 0 {
		candidateStr = fmt.Sprintf("\nCandidate nix attributes: %s", strings.Join(candidates, ", "))
	}

	prompt := fmt.Sprintf(`You are a Nix package resolver. Given a package name and ecosystem, return the exact nixpkgs attribute path.

Package: %s
Ecosystem: %s%s

Reply with ONLY the nix attribute path (e.g., "python3Packages.flask" or "nodePackages.typescript"). No explanation.`, pkgName, ecosystem, candidateStr)

	return l.ask(l.cfg.FastModel, prompt)
}

func (l *LLMClient) GenerateFlake(context string) (string, error) {
	prompt := fmt.Sprintf(`Generate a Nix flake.nix for the following project. Rules:
- No comments in the output
- Use exact versions where provided
- Clean minimal flake with devShell
- Only output the flake.nix content, nothing else

%s`, context)

	resp, err := l.ask(l.cfg.SmartModel, prompt)
	if err != nil {
		return "", err
	}
	return cleanLLMResponse(resp), nil
}

func (l *LLMClient) FixFlake(flakeContent, errorMsg string) (string, error) {
	prompt := fmt.Sprintf(`Fix this flake.nix that has an error. 

Common causes of errors:
- Missing 'python3Packages.X' often means X is a Python standard library module (like ctypes, abc, etc.) and should be REMOVED from buildInputs.
- Typos in package names.
- Packages not available in the current nixpkgs version.

OUTPUT FORMAT REQUIREMENTS:
- You MUST return ONLY the raw, exact, fully functioning Nix code.
- DO NOT wrap the code in markdown blocks (no `+"```"+`nix or `+"```"+`).
- DO NOT include ANY explanations or conversational text.
- If the error is about a lock file or something outside flake.nix, just return the exact original flake.nix back! Do NOT try to explain!

Error:
%s

Current flake.nix:
%s`, errorMsg, flakeContent)

	resp, err := l.ask(l.cfg.SmartModel, prompt)
	if err != nil {
		return "", err
	}
	return cleanLLMResponse(resp), nil
}

// RepairPackage asks the fast model to find the correct nix attribute for a broken package.
// Returns the correct attribute, or "SKIP" if the package should not be in the flake.
func (l *LLMClient) RepairPackage(badAttr, ecosystem, errorMsg string) (string, error) {
	prompt := fmt.Sprintf(`Package "%s" (%s) failed in nixpkgs: %s

Reply with ONLY one of:
1. The correct nixpkgs attribute (e.g. python313Packages.flask)
2. SKIP — if it's a stdlib module, or not available in nixpkgs and should be installed via pip/npm/cargo instead

One line. No explanation.`, badAttr, ecosystem, errorMsg)

	return l.ask(l.cfg.FastModel, prompt)
}

func cleanLLMResponse(resp string) string {
	resp = strings.TrimSpace(resp)
	if strings.HasPrefix(resp, "```") {
		// Remove first line (the opening ```nix or similar)
		lines := strings.Split(resp, "\n")
		if len(lines) > 2 {
			// Find closing ```
			var endIdx int
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
					endIdx = i
					break
				}
			}
			if endIdx > 1 {
				return strings.Join(lines[1:endIdx], "\n")
			}
		}
	}
	return resp
}

func (l *LLMClient) ask(model, prompt string) (string, error) {
	switch l.cfg.APIProvider {
	case "openrouter":
		return l.askOpenRouter(model, prompt)
	case "google":
		return l.askGoogle(model, prompt)
	default:
		return l.askOpenRouter(model, prompt)
	}
}

func (l *LLMClient) askOpenRouter(model, prompt string) (string, error) {
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.1,
		"max_tokens":  4096,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.cfg.APIKey)

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("openrouter error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

func (l *LLMClient) askGoogle(model, prompt string) (string, error) {
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, l.cfg.APIKey)

	body := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature":     0.1,
			"maxOutputTokens": 4096,
		},
	}

	data, _ := json.Marshal(body)
	resp, err := l.httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("google api request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("google api error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from google model")
	}

	return strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text), nil
}
