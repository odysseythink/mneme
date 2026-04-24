package vectordb

import (
	"context"
	"fmt"
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/odysseythink/mlog"
	"github.com/qdrant/go-client/qdrant"
	"github.com/ranwei/claude-context/pkg"
)

type QdrantStore struct {
	url         string
	collection  string
	client      *qdrant.Client
	vectorSize  uint64
	ready       bool
	mu          sync.Mutex
}

func NewQdrantStore(url, collection string) *QdrantStore {
	return &QdrantStore{
		url:        url,
		collection: collection,
	}
}

func (s *QdrantStore) Initialize(dbPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse URL to extract host and port
	host, port, err := s.parseURL(s.url)
	if err != nil {
		return fmt.Errorf("failed to parse Qdrant URL: %w", err)
	}

	// Create Qdrant client with gRPC configuration
	config := &qdrant.Config{
		Host: host,
		Port: port,
	}

	if s.client != nil {
		s.client.Close()
	}
	client, err := qdrant.NewClient(config)
	if err != nil {
		return fmt.Errorf("failed to create Qdrant client: %w", err)
	}
	s.client = client

	// Check if collection exists
	exists, err := client.CollectionExists(context.Background(), s.collection)
	if err != nil {
		return fmt.Errorf("failed to check collection existence: %w", err)
	}

	if exists {
		// Get collection info to determine vector size
		info, err := client.GetCollectionInfo(context.Background(), s.collection)
		if err != nil {
			return fmt.Errorf("failed to get collection info: %w", err)
		}

		if info.Config != nil && info.Config.Params != nil && info.Config.Params.VectorsConfig != nil {
			if params := info.Config.Params.VectorsConfig.GetParams(); params != nil {
				s.vectorSize = params.Size
			}
		}
		s.ready = true
		mlog.Infof("Qdrant collection '%s' exists with vector size %d", s.collection, s.vectorSize)
	} else {
		// Collection doesn't exist yet - will create on first insert
		mlog.Infof("Qdrant collection '%s' does not exist; will create on first insert", s.collection)
	}

	return nil
}

func (s *QdrantStore) parseURL(rawURL string) (host string, port int, err error) {
	// Default values
	host = "localhost"
	port = 6334 // gRPC port

	// Try to parse as URL with scheme
	if strings.Contains(rawURL, "://") {
		u, err := url.Parse(rawURL)
		if err != nil {
			return "", 0, fmt.Errorf("invalid URL: %w", err)
		}

		parts := strings.Split(u.Host, ":")
		host = parts[0]

		if len(parts) > 1 {
			// HTTP port provided (6333 -> gRPC port 6334)
			httpPort, err := strconv.Atoi(parts[1])
			if err != nil {
				return "", 0, fmt.Errorf("invalid port: %w", err)
			}
			// Convert HTTP port 6333 to gRPC port 6334
			if httpPort == 6333 {
				port = 6334
			} else {
				// Assume it's already the gRPC port
				port = httpPort
			}
		}
	} else {
		// Try to parse as host:port
		parts := strings.Split(rawURL, ":")
		host = parts[0]

		if len(parts) > 1 {
			httpPort, err := strconv.Atoi(parts[1])
			if err != nil {
				return "", 0, fmt.Errorf("invalid port: %w", err)
			}
			// Convert HTTP port 6333 to gRPC port 6334
			if httpPort == 6333 {
				port = 6334
			} else {
				port = httpPort
			}
		}
	}

	return host, port, nil
}

func (s *QdrantStore) InsertVector(ctx context.Context, vec pkg.Vector) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == nil {
		return fmt.Errorf("Qdrant client not initialized")
	}

	// If collection doesn't exist, create it on first insert
	if !s.ready {
		if err := s.createCollection(ctx, uint64(len(vec.Embedding))); err != nil {
			return err
		}
		s.ready = true
		s.vectorSize = uint64(len(vec.Embedding))
	}

	// Generate point ID using FNV hash of "FilePath:StartLine:EndLine"
	pointID := s.hashPointID(vec.FilePath, vec.StartLine, vec.EndLine)

	// Build payload
	payload := map[string]*qdrant.Value{
		"text":          qdrant.NewValueString(vec.Text),
		"file_path":     qdrant.NewValueString(vec.FilePath),
		"language":      qdrant.NewValueString(vec.Language),
		"start_line":    qdrant.NewValueInt(int64(vec.StartLine)),
		"end_line":      qdrant.NewValueInt(int64(vec.EndLine)),
		"codebase_hash": qdrant.NewValueString(vec.CodebaseHash),
		"indexed_at":    qdrant.NewValueString(vec.IndexedAt.Format(time.RFC3339)),
	}

	// Build point with vectors
	point := &qdrant.PointStruct{
		Id:      qdrant.NewIDNum(pointID),
		Vectors: qdrant.NewVectorsDense(vec.Embedding),
		Payload: payload,
	}

	// Upsert the point
	req := &qdrant.UpsertPoints{
		CollectionName: s.collection,
		Points:         []*qdrant.PointStruct{point},
		Wait:           ptrBool(true),
	}

	_, err := s.client.Upsert(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

func (s *QdrantStore) Search(ctx context.Context, embedding []float32, topK int) ([]pkg.Vector, error) {
	s.mu.Lock()
	if !s.ready || s.client == nil {
		s.mu.Unlock()
		return nil, nil
	}
	client := s.client
	collection := s.collection
	s.mu.Unlock()

	// Build query with vector
	query := qdrant.NewQuery(embedding...)
	scoreThreshold := float32(0.5)

	// Build QueryPoints request with with_vectors enabled
	req := &qdrant.QueryPoints{
		CollectionName: collection,
		Query:          query,
		Limit:          ptrUint64(uint64(topK)),
		ScoreThreshold: &scoreThreshold,
		WithVectors:    qdrant.NewWithVectors(true),
	}

	// Execute query
	results, err := client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to query: %w", err)
	}

	var vectors []pkg.Vector
	for _, point := range results {
		if point == nil {
			continue
		}

		// Extract embedding from point's vectors
		var embedding []float32
		if point.Vectors != nil {
			if denseVec := point.Vectors.GetVector(); denseVec != nil {
				if dense := denseVec.GetDense(); dense != nil {
					embedding = dense.Data
				}
			}
		}

		// Extract payload fields
		payload := point.Payload
		if payload == nil {
			mlog.Warningf("qdrant: skipping result with nil payload")
			continue
		}

		text := getStringValue(payload, "text")
		filePath := getStringValue(payload, "file_path")
		language := getStringValue(payload, "language")
		codebaseHash := getStringValue(payload, "codebase_hash")
		indexedAtStr := getStringValue(payload, "indexed_at")

		startLine := int(getIntValue(payload, "start_line"))
		endLine := int(getIntValue(payload, "end_line"))

		indexedAt, err := time.Parse(time.RFC3339, indexedAtStr)
		if err != nil {
			mlog.Warningf("qdrant: skipping result with unparseable indexed_at %q: %v", indexedAtStr, err)
			continue
		}

		vectors = append(vectors, pkg.Vector{
			Embedding:    embedding,
			Text:         text,
			FilePath:     filePath,
			Language:     language,
			StartLine:    startLine,
			EndLine:      endLine,
			CodebaseHash: codebaseHash,
			IndexedAt:    indexedAt,
		})
	}

	return vectors, nil
}

func (s *QdrantStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

func (s *QdrantStore) createCollection(ctx context.Context, vectorSize uint64) error {
	vectorParams := &qdrant.VectorParams{
		Size:     vectorSize,
		Distance: qdrant.Distance_Cosine,
	}

	vectorsConfig := qdrant.NewVectorsConfig(vectorParams)

	req := &qdrant.CreateCollection{
		CollectionName: s.collection,
		VectorsConfig:  vectorsConfig,
	}

	if err := s.client.CreateCollection(ctx, req); err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	mlog.Infof("Qdrant collection '%s' created with vector size %d", s.collection, vectorSize)
	return nil
}

func (s *QdrantStore) hashPointID(filePath string, startLine, endLine int) uint64 {
	h := fnv.New64a()
	h.Write([]byte(fmt.Sprintf("%s:%d:%d", filePath, startLine, endLine)))
	return h.Sum64()
}

// Helper functions for payload extraction
func getStringValue(payload map[string]*qdrant.Value, key string) string {
	if v, ok := payload[key]; ok && v != nil {
		return v.GetStringValue()
	}
	return ""
}

func getIntValue(payload map[string]*qdrant.Value, key string) int64 {
	if v, ok := payload[key]; ok && v != nil {
		return v.GetIntegerValue()
	}
	return 0
}

// Helper function to create pointer to bool
func ptrBool(b bool) *bool {
	return &b
}

// Helper function to create pointer to uint64
func ptrUint64(u uint64) *uint64 {
	return &u
}
