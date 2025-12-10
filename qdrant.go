package main

import (
	"context"
	"fmt"
	"log"

	"github.com/kelindar/search"
	"github.com/qdrant/go-client/qdrant"
)

func GetOperation(qc *qdrant.Client, text *string, embedder *search.Vectorizer) string {
	if text != nil {
		embeddedMsg, err := embedder.EmbedText(*text)
		if err != nil {
			log.Printf("Error embedding text: %v", err)
			panic(err)
		}
		result, err := qc.Query(context.Background(), &qdrant.QueryPoints{
			CollectionName: "gc_operations",
			Query:          qdrant.NewQuery(embeddedMsg...),
			WithPayload:    qdrant.NewWithPayload(true),
		})
		if err != nil {
			fmt.Println("Error querying Qdrant:", err)
			panic(err)
		}
		payload := result[0].GetPayload()
		if operationValue, exists := payload["operation"]; exists {
			// The value is a *qdrant.Value - we need to get the string from it
			qdrantValue := operationValue

			// Check if it has a string value and extract it
			if qdrantValue.GetStringValue() != "" {
				operation := qdrantValue.GetStringValue()
				fmt.Println(operation) // Prints: Delete
				return operation
			}
		}
	}
	return ""
}
