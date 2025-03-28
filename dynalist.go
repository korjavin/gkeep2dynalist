package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

const (
	dynalistAPIURL = "https://dynalist.io/api/v1/inbox/add"
	maxRetries     = 5                // Maximum number of retries
	minDelay       = 2 * time.Second  // Minimum delay between retries
	maxDelay       = 60 * time.Second // Maximum delay between retries
	minPause       = 1 * time.Second  // Minimum random pause between API calls
	maxPause       = 3 * time.Second  // Maximum random pause between API calls
)

// DynalistRequest represents the request body for the Dynalist API
type DynalistRequest struct {
	Token    string `json:"token"`
	Index    int    `json:"index,omitempty"`
	Content  string `json:"content"`
	Note     string `json:"note,omitempty"`
	Checked  bool   `json:"checked,omitempty"`
	Checkbox bool   `json:"checkbox,omitempty"`
}

// DynalistResponse represents the response from the Dynalist API
type DynalistResponse struct {
	Code    string `json:"_code"`
	Message string `json:"_msg,omitempty"`
	FileID  string `json:"file_id,omitempty"`
	NodeID  string `json:"node_id,omitempty"`
	Index   int    `json:"index,omitempty"`
}

// RetryStats tracks retry statistics
type RetryStats struct {
	TotalCalls      int
	SuccessfulCalls int
	FailedCalls     int
	Retries         int
	LastError       string
	LastStatus      string
}

// Global retry statistics
var Stats RetryStats

// AddToDynalist sends a message to the Dynalist inbox with retry logic
func AddToDynalist(token, content string, note string) error {
	// Add random pause before API call to avoid rate limiting
	randomPause := minPause + time.Duration(rand.Int63n(int64(maxPause-minPause)))
	time.Sleep(randomPause)

	// Create request body
	reqBody := DynalistRequest{
		Token:   token,
		Content: content,
		Note:    note,
	}

	// Marshal request body to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Initialize retry variables
	var lastErr error
	retryCount := 0
	Stats.TotalCalls++

	// Retry loop with exponential backoff
	for retryCount <= maxRetries {
		// Create HTTP request
		req, err := http.NewRequest("POST", dynalistAPIURL, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		// Send request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to send request: %w", err)
			Stats.LastError = lastErr.Error()
			retryCount++
			Stats.Retries++

			// If we've reached max retries, break
			if retryCount > maxRetries {
				break
			}

			// Calculate backoff delay with jitter
			delay := calculateBackoff(retryCount)
			time.Sleep(delay)
			continue
		}

		// Make sure we close the response body
		responseBody := resp.Body
		defer responseBody.Close()

		// Parse response
		var dynalistResp DynalistResponse
		if err := json.NewDecoder(responseBody).Decode(&dynalistResp); err != nil {
			lastErr = fmt.Errorf("failed to decode response: %w", err)
			Stats.LastError = lastErr.Error()
			retryCount++
			Stats.Retries++

			// If we've reached max retries, break
			if retryCount > maxRetries {
				break
			}

			// Calculate backoff delay with jitter
			delay := calculateBackoff(retryCount)
			time.Sleep(delay)
			continue
		}

		// Check response code
		if dynalistResp.Code == "Ok" {
			// Success!
			Stats.SuccessfulCalls++
			Stats.LastStatus = "Success"
			return nil
		}

		// Handle specific error codes
		lastErr = fmt.Errorf("dynalist API error: %s", dynalistResp.Code)
		if dynalistResp.Message != "" {
			lastErr = fmt.Errorf("dynalist API error: %s", dynalistResp.Message)
		}
		Stats.LastError = lastErr.Error()

		// If not a rate limit error, we might not want to retry
		if dynalistResp.Code != "TooManyRequests" && retryCount >= 2 {
			break
		}

		// Increment retry counter
		retryCount++
		Stats.Retries++

		// If we've reached max retries, break
		if retryCount > maxRetries {
			break
		}

		// Calculate backoff delay with jitter
		delay := calculateBackoff(retryCount)
		time.Sleep(delay)
	}

	// If we get here, all retries failed
	Stats.FailedCalls++
	Stats.LastStatus = "Failed"
	return lastErr
}

// calculateBackoff calculates exponential backoff with jitter
func calculateBackoff(retry int) time.Duration {
	// Calculate exponential backoff: minDelay * 2^retry
	backoff := float64(minDelay) * math.Pow(2, float64(retry))

	// Add jitter: random value between 0.5 and 1.5 of the calculated backoff
	jitter := 0.5 + rand.Float64()
	backoff = backoff * jitter

	// Cap at maxDelay
	if backoff > float64(maxDelay) {
		backoff = float64(maxDelay)
	}

	return time.Duration(backoff)
}
