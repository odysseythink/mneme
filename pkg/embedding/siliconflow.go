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

type SiliconFlowProvider struct {
	apiKey string
	model  string
	client *http.Client
}

func NewSiliconFlowProvider(apiKey, model string) *SiliconFlowProvider {
	return &SiliconFlowProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

type SiliconFlowRequest struct {
	Model    string   `json:"model"`
	Input    []string `json:"input"`
	Encoding string   `json:"encoding_format,omitempty"`
}

type SiliconFlowResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (p *SiliconFlowProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	req := SiliconFlowRequest{
		Model:    p.model,
		Input:    []string{text},
		Encoding: "float",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.siliconflow.cn/v1/embeddings", bytes.NewBuffer(body))
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

	var sfResp SiliconFlowResponse
	if err := json.Unmarshal(respBody, &sfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if sfResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", sfResp.Error.Message)
	}

	if len(sfResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return sfResp.Data[0].Embedding, nil
}

func (p *SiliconFlowProvider) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	req := SiliconFlowRequest{
		Model:    p.model,
		Input:    texts,
		Encoding: "float",
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://api.siliconflow.cn/v1/embeddings", bytes.NewBuffer(body))
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

	var sfResp SiliconFlowResponse
	if err := json.Unmarshal(respBody, &sfResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if sfResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", sfResp.Error.Message)
	}

	result := make([][]float32, len(texts))
	for _, data := range sfResp.Data {
		if data.Index < len(result) {
			result[data.Index] = data.Embedding
		}
	}

	return result, nil
}
