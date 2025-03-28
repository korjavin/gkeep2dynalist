package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProgressStats tracks processing progress
type ProgressStats struct {
	TotalNotes     int
	ProcessedNotes int
	SkippedNotes   int
	StartTime      time.Time
}

// Global progress statistics
var Progress ProgressStats

func init() {
	// Seed the random number generator
	rand.Seed(time.Now().UnixNano())

	// Initialize progress tracking
	Progress = ProgressStats{
		StartTime: time.Now(),
	}
}

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

	// Count total notes first
	countJsonFiles(*takeoutPath)
	log.Printf("Found %d total JSON files to process", Progress.TotalNotes)

	// Process Google Keep folder
	err = processKeepFolder(*takeoutPath, dynalistToken, r2Client)
	if err != nil {
		log.Fatalf("Error processing Google Keep folder: %v", err)
	}

	// Display final statistics
	duration := time.Since(Progress.StartTime).Round(time.Second)
	log.Printf("Successfully processed %d/%d Google Keep notes in %s",
		Progress.ProcessedNotes, Progress.TotalNotes, duration)
	log.Printf("Skipped %d notes (archived or errors)", Progress.SkippedNotes)
	log.Printf("API Stats: %d successful, %d failed, %d retries",
		Stats.SuccessfulCalls, Stats.FailedCalls, Stats.Retries)
}

// countJsonFiles counts the total number of JSON files in the folder
func countJsonFiles(folderPath string) {
	filepath.Walk(folderPath, func(filePath string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !fileInfo.IsDir() && filepath.Ext(filePath) == ".json" {
			Progress.TotalNotes++
		}
		return nil
	})
}

// displayProgress shows the current progress
func displayProgress() {
	percent := float64(Progress.ProcessedNotes) / float64(Progress.TotalNotes) * 100
	elapsed := time.Since(Progress.StartTime).Round(time.Second)

	// Create a simple progress bar
	width := 30
	completed := int(float64(width) * float64(Progress.ProcessedNotes) / float64(Progress.TotalNotes))
	bar := strings.Repeat("=", completed) + strings.Repeat(" ", width-completed)

	fmt.Printf("\r[%s] %.1f%% (%d/%d) | Elapsed: %s | API: %d ok, %d fail, %d retry | %s",
		bar, percent, Progress.ProcessedNotes, Progress.TotalNotes,
		elapsed, Stats.SuccessfulCalls, Stats.FailedCalls, Stats.Retries,
		Stats.LastStatus)
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
			Progress.SkippedNotes++
			displayProgress()
			return nil // Continue processing other files
		}

		// Ignore archived notes
		if note.IsArchived {
			log.Printf("Ignoring archived note: %s", filePath)
			Progress.SkippedNotes++
			displayProgress()
			return nil
		}

		// Process the message
		err = processMessage(note, folderPath, dynalistToken, r2Client, filePath)
		if err != nil {
			log.Printf("Failed to process message: %v", err)
			Progress.SkippedNotes++
			displayProgress()
			return nil // Continue processing other files
		}

		// Update progress
		Progress.ProcessedNotes++
		displayProgress()
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
	// Tags will now go in the title, not in the note content

	// Set the title
	title := note.Title
	if title == "" {
		// Use shortened filename
		baseTitle := shortenFilename(filePath)

		// Add first few lines of content to title if available
		if note.TextContent != "" {
			// Get first few lines of content
			contentLines := strings.Split(note.TextContent, "\n")
			previewText := ""

			// Take up to 2 non-empty lines for the preview
			lineCount := 0
			for _, line := range contentLines {
				trimmedLine := strings.TrimSpace(line)
				if trimmedLine != "" {
					if previewText != "" {
						previewText += " | "
					}
					// Limit each line to 30 chars
					if len(trimmedLine) > 30 {
						previewText += trimmedLine[:30] + "..."
					} else {
						previewText += trimmedLine
					}

					lineCount++
					if lineCount >= 2 {
						break
					}
				}
			}

			if previewText != "" {
				title = baseTitle + ": " + previewText
			} else {
				title = baseTitle
			}
		} else {
			title = baseTitle
		}
	}

	// Add prefix and tags to title
	title = "gkeep: " + title
	if hashtags != "" {
		title += " " + hashtags
	}

	// Forward the message to Dynalist
	err := AddToDynalist(dynalistToken, title, noteContent)
	if err != nil {
		log.Printf("Failed to add message to Dynalist: %v", err)
		return err
	}

	return nil
}
