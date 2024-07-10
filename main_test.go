package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

var localTestContainer *LocalTestContainer

func TestMain(m *testing.M) {
	var err error
	localTestContainer, err = CreateLocalTestContainer()
	if err != nil {
		fmt.Printf("Error initializing Docker localTestContainer: %s", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func(p string) {
		err := waitForServiceToBeReady(p)
		if err != nil {
			localTestContainer.Close()
			panic(fmt.Errorf("Error waiting for local container to start: %w", err))
		}
		wg.Done()
	}(localTestContainer.appport)

	wg.Wait()

	result := m.Run()

	localTestContainer.Close()
	os.Exit(result)
}

func waitForServiceToBeReady(port string) error {
	limit := 30
	wait := 250 * time.Millisecond
	started := time.Now()
	url := fmt.Sprintf("http://localhost:%s/%s", port, "health")

	for i := 0; i < limit; i++ {
		getReq, _ := http.NewRequest("GET", url, nil)
		client := http.DefaultClient
		getResp, err := client.Do(getReq)
		if err != nil {
			time.Sleep(wait)
			continue
		}

		if getResp.StatusCode == 200 {
			return nil
		}
		defer getResp.Body.Close()
	}
	return fmt.Errorf("the health endpoint didn't respond successfully within %f seconds.", time.Since(started).Seconds())
}

func TestCreateItem(t *testing.T) {
	// Create an item to test retrieval
	newItem := Item{
		Name:  "Testitem",
		Price: 201,
	}
	url := fmt.Sprintf("http://localhost:%s/%s", localTestContainer.appport, "items")
	jsonValue, _ := json.Marshal(newItem)
	createReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	createReq.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer createResp.Body.Close()

	var createdItem Item
	json.NewDecoder(createResp.Body).Decode(&createdItem)

	// Test retrieving the created item
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%d", url, createdItem.ID), nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetchedItem Item
	json.NewDecoder(getResp.Body).Decode(&fetchedItem)

	assert.Equal(t, createdItem.Name, fetchedItem.Name)
	assert.Equal(t, createdItem.Price, fetchedItem.Price)
}

func TestGetItem(t *testing.T) {
	// Create an item to test retrieval
	newItem := Item{
		Name:  "TestGetItem",
		Price: 200,
	}
	url := fmt.Sprintf("http://localhost:%s/%s", localTestContainer.appport, "items")
	jsonValue, _ := json.Marshal(newItem)
	createReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	createReq.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer createResp.Body.Close()

	var createdItem Item
	json.NewDecoder(createResp.Body).Decode(&createdItem)

	// Test retrieving the created item
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%d", url, createdItem.ID), nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetchedItem Item
	json.NewDecoder(getResp.Body).Decode(&fetchedItem)

	assert.Equal(t, createdItem.Name, fetchedItem.Name)
	assert.Equal(t, createdItem.Price, fetchedItem.Price)
}

func TestUpdateItem(t *testing.T) {
	// Create an item to test updating
	newItem := Item{
		Name:  "TestUpdateItem",
		Price: 300,
	}
	url := fmt.Sprintf("http://localhost:%s/%s", localTestContainer.appport, "items")
	jsonValue, _ := json.Marshal(newItem)
	createReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	createReq.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer createResp.Body.Close()

	var createdItem Item
	json.NewDecoder(createResp.Body).Decode(&createdItem)

	// Test updating the created item
	updateItem := Item{
		Name:  "UpdatedItem",
		Price: 400,
	}
	updateValue, _ := json.Marshal(updateItem)
	updateReq, _ := http.NewRequest("PUT", fmt.Sprintf("%s/%d", url, createdItem.ID), bytes.NewBuffer(updateValue))
	updateReq.Header.Set("Content-Type", "application/json")

	updateResp, err := client.Do(updateReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer updateResp.Body.Close()

	assert.Equal(t, http.StatusOK, updateResp.StatusCode)

	var updatedItem Item
	json.NewDecoder(updateResp.Body).Decode(&updatedItem)

	assert.Equal(t, updateItem.Name, updatedItem.Name)
	assert.Equal(t, updateItem.Price, updatedItem.Price)
}

func TestDeleteItem(t *testing.T) {
	// Create an item to test deletion
	newItem := Item{
		Name:  "TestDeleteItem",
		Price: 500,
	}
	url := fmt.Sprintf("http://localhost:%s/%s", localTestContainer.appport, "items")
	jsonValue, _ := json.Marshal(newItem)
	createReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonValue))
	createReq.Header.Set("Content-Type", "application/json")

	client := http.DefaultClient
	createResp, err := client.Do(createReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer createResp.Body.Close()

	var createdItem Item
	json.NewDecoder(createResp.Body).Decode(&createdItem)

	// Test deleting the created item
	deleteReq, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/%d", url, createdItem.ID), nil)
	deleteResp, err := client.Do(deleteReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer deleteResp.Body.Close()

	assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)

	// Verify item deletion
	getReq, _ := http.NewRequest("GET", fmt.Sprintf("%s/%d", url, createdItem.ID), nil)
	getResp, err := client.Do(getReq)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer getResp.Body.Close()

	assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
}
