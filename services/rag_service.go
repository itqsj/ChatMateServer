package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	qdrantindexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
	qdrantretriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	qdrantclient "github.com/qdrant/go-client/qdrant"
)

var (
	embedder        embedding.Embedder
	qdrantIndexer   indexer.Indexer
	qdrantRetriever retriever.Retriever
	collectionName  = "knowledge_base"
)

func InitRAG() error {
	var err error
	ctx := context.Background()

	// 1. 初始化 Zhipu (OpenAI-compatible) Embedder
	apiKey := os.Getenv("KNOWLEDGE_MODEL")
	embedder, err = openai.NewEmbedder(ctx, &openai.EmbeddingConfig{
		BaseURL: "https://open.bigmodel.cn/api/paas/v4/",
		APIKey:  apiKey,
		Model:   "embedding-2",
	})
	if err != nil {
		return fmt.Errorf("init embedder failed: %w", err)
	}

	// 2. 初始化 Qdrant 客户端
	qdrantURL := os.Getenv("QDRANT_URL")
	if qdrantURL == "" {
		qdrantURL = "localhost:6334" // Default gRPC port
	} else {
		qdrantURL = strings.Replace(qdrantURL, "http://", "", 1)
		qdrantURL = strings.Replace(qdrantURL, ":6333", ":6334", 1) // Qdrant-go uses gRPC by default
	}
	host := strings.Split(qdrantURL, ":")[0]

	qClient, err := qdrantclient.NewClient(&qdrantclient.Config{
		Host:   host,
		Port:   6334,
		UseTLS: false,
	})
	if err != nil {
		return fmt.Errorf("init qdrant client failed: %w", err)
	}

	// 检查并创建集合
	exists, err := qClient.CollectionExists(ctx, collectionName)
	if err != nil {
		return fmt.Errorf("check collection exists failed: %w", err)
	}
	if !exists {
		err = qClient.CreateCollection(ctx, &qdrantclient.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: qdrantclient.NewVectorsConfig(&qdrantclient.VectorParams{
				Size:     1024,
				Distance: qdrantclient.Distance_Cosine,
			}),
		})
		if err != nil {
			return fmt.Errorf("create collection failed: %w", err)
		}
	}

	// 3. 初始化 Indexer
	qdrantIndexer, err = qdrantindexer.NewIndexer(ctx, &qdrantindexer.Config{
		Client:     qClient,
		Collection: collectionName,
		Embedding:  embedder,
	})
	if err != nil {
		return fmt.Errorf("init indexer failed: %w", err)
	}

	// 4. 初始化 Retriever
	qdrantRetriever, err = qdrantretriever.NewRetriever(ctx, &qdrantretriever.Config{
		Client:     qClient,
		Collection: collectionName,
		Embedding:  embedder,
		TopK:       3,
	})
	if err != nil {
		return fmt.Errorf("init retriever failed: %w", err)
	}

	return nil
}

// 简单按段落切分
func splitMarkdown(content string) []string {
	parts := strings.Split(content, "\n\n")
	var chunks []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 0 {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

func ProcessAndStoreMarkdown(file *multipart.FileHeader) error {
	f, err := file.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	chunks := splitMarkdown(string(content))
	ctx := context.Background()

	// 生成 Embeddings
	embeddings, err := embedder.EmbedStrings(ctx, chunks)
	if err != nil {
		return fmt.Errorf("embed strings failed: %w", err)
	}

	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embeddings count %d mismatch chunks count %d", len(embeddings), len(chunks))
	}

	var docs []*schema.Document
	for i, chunk := range chunks {
		doc := &schema.Document{
			Content: chunk,
			MetaData: map[string]any{
				"filename": file.Filename,
			},
		}
		doc = doc.WithDenseVector(embeddings[i])
		docs = append(docs, doc)
	}

	// 存入 Qdrant
	_, err = qdrantIndexer.Store(ctx, docs)
	if err != nil {
		return fmt.Errorf("indexer store failed: %w", err)
	}

	return nil
}

func RetrieveContext(query string) (string, error) {
	ctx := context.Background()
	docs, err := qdrantRetriever.Retrieve(ctx, query)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, doc := range docs {
		sb.WriteString(doc.Content)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
