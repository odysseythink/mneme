package tests

import (
	"testing"

	"github.com/ranwei/claude-context/pkg/embedding"
)

func TestSiliconFlowProviderInit(t *testing.T) {
	provider := embedding.NewSiliconFlowProvider("sk-test", "BAAI/bge-large-zh-v1.5")
	if provider == nil {
		t.Fatal("Failed to create SiliconFlow provider")
	}
}
