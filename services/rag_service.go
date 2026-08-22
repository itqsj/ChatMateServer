package services

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"strings"

	"github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	"github.com/cloudwego/eino-ext/components/embedding/openai"
	qdrantindexer "github.com/cloudwego/eino-ext/components/indexer/qdrant"
	qdrantretriever "github.com/cloudwego/eino-ext/components/retriever/qdrant"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
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

	// 4. 初始化 Retriever (固定写法/标准用法)
	// 作用：根据传入的配置参数生成 Retriever 实例，用于后续在问答时进行向量相似度检索。
	qdrantRetriever, err = qdrantretriever.NewRetriever(ctx, &qdrantretriever.Config{
		Client:     qClient,        // Qdrant 客户端：用于与 Qdrant 向量数据库实例进行 gRPC 通信
		Collection: collectionName, // 目标集合：指定在 Qdrant 中的哪一个 collection 内执行特征检索
		Embedding:  embedder,       // 向量编码器：用于自动将问答时的用户 query 文本转换成对应的浮点数向量
		TopK:       10,             // 召回条数：检索时返回相似度最高的 Top N 个文档分片（调大为 10 可防止 h4 级细粒度分片丢失细节）
	})
	if err != nil {
		return fmt.Errorf("init retriever failed: %w", err)
	}

	return nil
}

func ProcessAndStoreMarkdown(file *multipart.FileHeader) error {
	f, err := file.Open()
	if err != nil {
		return err
	}
	defer f.Close()

	// 读取上传的 Markdown 文件的全部文本内容
	content, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// 1. 将读取的原始文件内容包装为初始的 schema.Document
	// 作用：将原始文本字符串与自定义元数据（这里是文件名）统一包装成 Eino 的文档对象，为后续的切分转换做准备。
	rawDoc := &schema.Document{
		Content: string(content),
		MetaData: map[string]any{
			"filename": file.Filename,
		},
	}

	// 2. 初始化 Eino Markdown Header Splitter Transformer (固定写法/标准用法)
	// 作用：利用 Eino 生态提供的切分组件，将大段 Markdown 结构化分割成细粒度的段落 Chunk。
	// 配置：
	// - Headers: 定义遇到哪些标题符号时进行切分，此处支持从 h1 (#) 到 h4 (####) 的细粒度层级。
	// - TrimHeaders: 设为 false 表示在拆分后的子片段中保留标题行本身，有助于增强向量表示的语义完整性。
	splitter, err := markdown.NewHeaderSplitter(ctx, &markdown.HeaderConfig{
		Headers: map[string]string{
			"#":   "h1",
			"##":  "h2",
			"###": "h3",
		},
		TrimHeaders: false,
	})
	if err != nil {
		return fmt.Errorf("create markdown splitter failed: %w", err)
	}

	// 3. 执行文档结构化切分 (固定写法)
	// 作用：将原始的单个大 Document 转换/切分为多个按标题组织的小 Document。
	// 特性：切分后，Eino 会在子文档 MetaData 中自动注入对应的标题层级元数据（如 "h1": "xxx"、"h2": "xxx" 等）。
	docs, err := splitter.Transform(ctx, []*schema.Document{rawDoc})
	if err != nil {
		return fmt.Errorf("split markdown failed: %w", err)
	}

	// 为每个拆分后的文档片段分配唯一 UUID 并进行标题路径前置增强 (固定写法/标准做法)
	// 作用：
	// 1. Qdrant 要求存入的每一个 Point 都有唯一的 ID，故分配 UUID 避免空主键报错。
	// 2. 将 MetaData 中的各级父标题（h1-h4）以 " > " 拼接并前缀挂载到正文头部。
	//    确保每个子片段在向量化（Embedding）时均包含章节上下文（如“6月”），极大提升搜索召回率。
	for _, doc := range docs {
		doc.ID = uuid.NewString()

		var headerParts []string
		for _, key := range []string{"h1", "h2", "h3", "h4"} {
			if val, ok := doc.MetaData[key].(string); ok && val != "" {
				headerParts = append(headerParts, val)
			}
		}
		if len(headerParts) > 0 {
			headerPrefix := strings.Join(headerParts, " > ") + "\n"
			doc.Content = headerPrefix + doc.Content
		}
	}

	// 4. 直接存入 Qdrant (固定写法/标准用法)
	// 作用：调用配置好的 Qdrant Indexer 将包含文本、业务元数据的文档切片批量持久化到 Qdrant 数据库中。
	// 特性：因为 Qdrant Indexer (qdrantIndexer) 已经配置了 Embedder，所以调用 Store 时，Eino 框架会
	// 自动在内部对 docs 中的文本块执行向量化（在内部调用 EmbedStrings 得到 []float64），
	// 然后安全地将其存入 Qdrant 向量数据库，同时保持原本的 doc.MetaData 不被 []float64 字段污染，避免序列化崩溃。
	// 此时 docs 数组中的每一个 Document 对象的字段与格式逻辑类似于：
	// &schema.Document{
	//     ID:      "d3b07384-d113-4ec6-a5d9-c3d52368c341", // UUID 字符串，在 Qdrant 中用作唯一标识 point_id
	//     Content: "## 二级标题\n正文段落...",                // 切片正文（TrimHeaders 为 false，因此包含标题行本身）
	//     MetaData: map[string]any{
	//         "filename": "xxx.md", // 从原始文档继承而来的文件名
	//         "h1":       "一级标题", // Eino Splitter 根据标题级别层级关系自动识别并抽取的元数据
	//         "h2":       "二级标题", // 同上，自动识别抽取的当前切片的二级标题名
	//     },
	// }
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
