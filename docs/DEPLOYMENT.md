# Deployment Guide

## Local Development

1. **Clone and build:**
   ```bash
   git clone <repo>
   cd go-mneme
   go build -o bin/mneme ./cmd/mcp
   ```

2. **Set credentials:**
   ```bash
   export EMBEDDING_API_KEY="sk-xxxx"
   export EMBEDDING_PROVIDER="siliconflow"
   ```

3. **Register with Claude Code:**
   ```bash
   claude mcp add mneme -- ./bin/mneme
   ```

## Docker Deployment (Future)

Dockerfile coming soon for containerized deployment.

## Performance Tuning

### Database

- For 100k+ vectors, create an index: `CREATE INDEX ON vectors(embedding)`
- DuckDB is single-threaded; for parallel indexing, use multiple processes with separate DBs

### Embedding Generation

- Batch queries of 20-50 texts for efficiency
- Use embedding caching to avoid re-querying identical text
- Implement incremental indexing to avoid re-processing unchanged files

## Troubleshooting

**"API key invalid"**
- Verify `EMBEDDING_API_KEY` is set correctly
- Check provider documentation for key format

**"Database locked"**
- Multiple processes accessing same DB; use separate DB paths
- Or implement connection pooling

**"Out of memory during indexing"**
- Reduce batch size (currently 20-50 per batch)
- Index in smaller chunks per run
- Consider incremental indexing for future versions
