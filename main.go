package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	// Define command-line flags
	takeoutPath := flag.String("takeout", "", "Path to the Google Keep takeout folder")
	flag.Parse()

	// Validate command-line arguments
	if *takeoutPath == "" {
		log.Fatal("Usage: gkeep2dynalist -takeout <takeout_path>")
	}

	// Validate that the provided path exists and is a directory
	fileInfo, err := os.Stat(*takeoutPath)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	if !fileInfo.IsDir() {
		log.Fatalf("Error: %s is not a directory", *takeoutPath)
	}

	// Get environment variables
	dynalistToken := os.Getenv("DYNALIST_TOKEN")

	// Validate environment variables
	if dynalistToken == "" {
		log.Fatal("DYNALIST_TOKEN environment variables must be set")
	}

	// Initialize Cloudflare R2 client if environment variables are set
	var r2Client *CloudflareR2Client
	if os.Getenv("CF_ACCOUNT_ID") != "" {
		r2Client, err = NewCloudflareR2Client()
		if err != nil {
			log.Printf("Warning: Failed to initialize Cloudflare R2 client: %v", err)
			log.Printf("Media uploads will be disabled")
		} else {
			log.Printf("Cloudflare R2 client initialized successfully")
		}
	} else {
		log.Printf("Cloudflare R2 environment variables not set, media uploads will be disabled")
	}

	// Process Google Keep folder
	err = processKeepFolder(*takeoutPath, dynalistToken, r2Client)
	if err != nil {
		log.Fatalf("Error processing Google Keep folder: %v", err)
	}

	log.Println("Successfully processed all Google Keep notes.")
}

func processKeepFolder(folderPath string, dynalistToken string, r2Client *CloudflareR2Client) error {
	// Walk through the folder
	return filepath.Walk(folderPath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if fileInfo.IsDir() {
			return nil
		}

		// Process only JSON files
		if filepath.Ext(filePath) != ".json" {
			return nil
		}

		// Parse the Keep Note
		note, err := parseKeepNote(filePath)
		if err != nil {
			log.Printf("Failed to parse Keep note: %v", err)
			return nil // Continue processing other files
		}

		// Ignore archived notes
		if note.IsArchived {
			log.Printf("Ignoring archived note: %s", filePath)
			return nil
		}

		// Process the message
		err = processMessage(note, folderPath, dynalistToken, r2Client, filePath)
		if err != nil {
			log.Printf("Failed to process message: %v", err)
			return nil // Continue processing other files
		}

		return nil
	})
}

func processMessage(note *KeepNote, folderPath string, dynalistToken string, r2Client *CloudflareR2Client, filePath string) error {
	var attachmentLinks []string
	// Process attachments
	if r2Client != nil && len(note.Attachments) > 0 {
		for _, attachment := range note.Attachments {
			attachmentFile, err := findAttachmentFile(folderPath, attachment.FilePath)
			if err != nil {
				log.Printf("Failed to find attachment file: %v", err)
				continue // Continue processing other attachments
			}

			r2URL, err := r2Client.UploadLocalFile(attachmentFile)
			if err != nil {
				log.Printf("Failed to upload attachment: %v", err)
				continue // Continue processing other attachments
			}

			attachmentLinks = append(attachmentLinks, fmt.Sprintf("[%s](%s)", attachment.FilePath, r2URL))
		}
	}

	// Process labels
	hashtags := processLabels(note.Labels)

	// Format the note content
	noteContent := note.TextContent
	if len(attachmentLinks) > 0 {
		noteContent += "\n\nAttachments:\n" + strings.Join(attachmentLinks, "\n")
	}
	if hashtags != "" {
		noteContent += "\n\n" + hashtags
	}

	// Set the title
	title := note.Title
	if title == "" {
		title = shortenFilename(filePath)
	}
	title = "gkeep: " + title

	// Forward the message to Dynalist
	err := AddToDynalist(dynalistToken, title, noteContent)
	if err != nil {
		log.Printf("Failed to add message to Dynalist: %v", err)
		return err
	}

	return nil
}
