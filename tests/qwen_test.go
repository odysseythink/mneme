package tests

import (
	"testing"

	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestQwenProviderInit(t *testing.T) {
	provider := embedding.NewQwenProvider("sk-test", "text-embedding-v2")
	if provider == nil {
		t.Fatal("Failed to create Qwen provider")
	}
}
