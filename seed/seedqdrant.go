package seed

import (
	"context"
	"fmt"

	"github.com/kelindar/search"
	"github.com/qdrant/go-client/qdrant"
)

func SeedGCOperations() {
	m, err := search.NewVectorizer("./dist/all-minilm-l6-v2-q8_0.gguf", 1)
	if err != nil {
		// handle error
		fmt.Println("hello")
	}

	embedAdd, err := m.EmbedText("Add event to my calendar")
	index := search.NewIndex[string]()
	index.Add(embedAdd, "Add event to my calendar")

	embedDelete, err := m.EmbedText("Delete event from my calendar")
	index.Add(embedDelete, "Delete event from my calendar")

	embedUpdate, err := m.EmbedText("Update event on my calendar")
	index.Add(embedUpdate, "Update event from my calendar")

	defer m.Close()

	client, err := qdrant.NewClient(&qdrant.Config{
		Host: "localhost",
		Port: 6334,
	})
	fmt.Println(len(embedAdd), len(embedDelete), len(embedUpdate))
	client.CreateCollection(context.Background(), &qdrant.CreateCollection{
		CollectionName: "gc_operations",
		VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
			Size:     384,
			Distance: qdrant.Distance_Cosine,
		}),
	})

	operationInfo, err := client.Upsert(context.Background(), &qdrant.UpsertPoints{
		CollectionName: "gc_operations",
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDNum(1),
				Vectors: qdrant.NewVectors(embedAdd...),
				Payload: qdrant.NewValueMap(map[string]any{"operation": "Add"}),
			},
			{
				Id:      qdrant.NewIDNum(2),
				Vectors: qdrant.NewVectors(embedDelete...),
				Payload: qdrant.NewValueMap(map[string]any{"operation": "Delete"}),
			},
			{
				Id:      qdrant.NewIDNum(3),
				Vectors: qdrant.NewVectors(embedUpdate...),
				Payload: qdrant.NewValueMap(map[string]any{"operation": "Update"}),
			},
		},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(operationInfo)

}
