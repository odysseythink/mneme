package embedding

import (
	"context"
	"crypto/md5"
	"fmt"
	"sync"

	"github.com/ranwei/claude-context/pkg"
)

type CachedClient struct {
	provider pkg.EmbeddingProvider
	cache    map[string][]float32
	mu       sync.RWMutex
}

func NewCachedClient(provider pkg.EmbeddingProvider) *CachedClient {
	return &CachedClient{
		provider: provider,
		cache:    make(map[string][]float32),
	}
}

func (c *CachedClient) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	hash := c.hashText(text)

	c.mu.RLock()
	if emb, found := c.cache[hash]; found {
		c.mu.RUnlock()
		return emb, nil
	}
	c.mu.RUnlock()

	emb, err := c.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cache[hash] = emb
	c.mu.Unlock()

	return emb, nil
}

func (c *CachedClient) hashText(text string) string {
	hash := md5.Sum([]byte(text))
	return fmt.Sprintf("%x", hash)
}

func (c *CachedClient) ClearCache() {
	c.mu.Lock()
	c.cache = make(map[string][]float32)
	c.mu.Unlock()
}
