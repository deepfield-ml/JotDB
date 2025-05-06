package main

import (
	jotdb "JotDB/V0.01"
	"fmt"
)

func main() {
	// Initialize the store
	store, err := jotdb.NewJotDB("./jotdb")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer store.Close()

	// Example JSON document
	doc := map[string]interface{}{
		"id":   "doc001",
		"data": map[string]interface{}{
			"name":  "Example",
			"score": 42.5,
			"tags":  []interface{}{"test", "demo"},
			"active": true,
			"meta":   nil,
		},
	}

	// Store the document
	if err := store.Store("doc001", doc); err != nil {
		fmt.Println("Error storing document:", err)
		return
	}

	// Retrieve the document
	var retrieved map[string]interface{}
	if err := store.Retrieve("doc001", &retrieved); err != nil {
		fmt.Println("Error retrieving document:", err)
		return
	}
	fmt.Printf("Retrieved document: %+v\n", retrieved)

	// Delete the document
	if err := store.Delete("doc001"); err != nil {
		fmt.Println("Error deleting document:", err)
		return
	}
}