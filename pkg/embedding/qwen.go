package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type QwenProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewQwenProvider(apiKey, model string) *QwenProvider {
	return &QwenProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type QwenRequest struct {
	Model      string     `json:"model"`
	Input      QwenInput  `json:"input"`
	Parameters Parameters `json:"parameters,omitempty"`
}

type QwenInput struct {
	Texts []string `json:"texts"`
}

type Parameters struct{}

type QwenResponse struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float32 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	RequestID string `json:"request_id"`
	Usage     struct {
		InputTokens int `json:"input_tokens"`
	} `json:"usage"`
}

func (p *QwenProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	req := QwenRequest{
		Model: p.model,
		Input: QwenInput{
			Texts: []string{text},
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var qResp QwenResponse
	if err := json.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(qResp.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return qResp.Output.Embeddings[0].Embedding, nil
}

func (p *QwenProvider) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := QwenRequest{
		Model: p.model,
		Input: QwenInput{
			Texts: texts,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://dashscope.aliyuncs.com/api/v1/services/embeddings/text-embedding/text-embedding", bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var qResp QwenResponse
	if err := json.Unmarshal(respBody, &qResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result := make([][]float32, len(texts))
	for _, emb := range qResp.Output.Embeddings {
		if emb.TextIndex < len(result) {
			result[emb.TextIndex] = emb.Embedding
		}
	}

	return result, nil
}
