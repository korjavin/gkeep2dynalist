package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// CloudflareR2Client represents a client for Cloudflare R2 storage
type CloudflareR2Client struct {
	s3Client   *s3.Client
	bucketName string
	accountID  string
}

// NewCloudflareR2Client creates a new Cloudflare R2 client
func NewCloudflareR2Client() (*CloudflareR2Client, error) {
	// Cloudflare R2 credentials
	accountID := os.Getenv("CF_ACCOUNT_ID")
	accessKeyID := os.Getenv("CF_ACCESS_KEY_ID")
	accessKeySecret := os.Getenv("CF_ACCESS_KEY_SECRET")
	bucketName := os.Getenv("CF_BUCKET_NAME")

	// Validate required environment variables
	if accountID == "" || accessKeyID == "" || accessKeySecret == "" || bucketName == "" {
		return nil, fmt.Errorf("missing required Cloudflare R2 environment variables")
	}

	// Initialize S3 client for Cloudflare R2
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKeySecret, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	return &CloudflareR2Client{
		s3Client:   s3Client,
		bucketName: bucketName,
		accountID:  accountID,
	}, nil
}

// GetDashboardURL returns the Cloudflare dashboard URL for an object
func (c *CloudflareR2Client) GetDashboardURL(objectPath string) string {
	return fmt.Sprintf("https://dash.cloudflare.com/%s/r2/default/buckets/%s/objects/%s",
		c.accountID, c.bucketName, objectPath)
}

// UploadFile uploads a file to Cloudflare R2 and returns the Cloudflare dashboard URL
func (c *CloudflareR2Client) UploadFile(fileData []byte, fileExt string) (string, error) {
	// Generate a unique filename
	timestamp := time.Now().UnixNano()
	fileName := fmt.Sprintf("%d%s", timestamp, fileExt)

	// Detect content type
	contentType := http.DetectContentType(fileData)

	// Upload to R2
	_, err := c.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(c.bucketName),
		Key:         aws.String(fileName),
		Body:        bytes.NewReader(fileData),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to R2: %w", err)
	}

	// Return the Cloudflare dashboard URL
	return fmt.Sprintf("https://dash.cloudflare.com/%s/r2/default/buckets/%s/objects/%s/details",
		c.accountID, c.bucketName, fileName), nil
}

// DownloadFileFromTelegram downloads a file from Telegram
func DownloadFileFromTelegram(fileURL string) ([]byte, error) {
	// Create HTTP client
	client := &http.Client{}

	// Create request
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: status code %d", resp.StatusCode)
	}

	// Read response body
	fileData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file data: %w", err)
	}

	return fileData, nil
}

// GetFileExtension returns the file extension based on content type
func GetFileExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}

// GetFileExtensionFromFilename returns the file extension from a filename
func GetFileExtensionFromFilename(filename string) string {
	return filepath.Ext(filename)
}

// UploadLocalFile uploads a local file to Cloudflare R2 and returns the Cloudflare dashboard URL
func (c *CloudflareR2Client) UploadLocalFile(filePath string) (string, error) {
	// Read the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Get the file extension
	fileExt := filepath.Ext(filePath)

	// Upload the file
	return c.UploadFile(fileData, fileExt)
}
