package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// KeepNote represents a Google Keep note from the takeout JSON
type KeepNote struct {
	Title                   string       `json:"title"`
	TextContent             string       `json:"textContent"`
	TextContentHTML         string       `json:"textContentHtml,omitempty"`
	Attachments             []Attachment `json:"attachments,omitempty"`
	Labels                  []Label      `json:"labels,omitempty"`
	UserEditedTimestampUsec int64        `json:"userEditedTimestampUsec"`
	CreatedTimestampUsec    int64        `json:"createdTimestampUsec"`
	IsArchived              bool         `json:"isArchived"` // Add IsArchived field
	// Other fields...
}

type Attachment struct {
	FilePath string `json:"filePath"`
	MimeType string `json:"mimetype"`
}

type Label struct {
	Name string `json:"name"`
}

// parseKeepNote parses a Google Keep JSON file into a KeepNote struct
func parseKeepNote(filePath string) (*KeepNote, error) {
	// Read the file
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Unmarshal the JSON data
	var note KeepNote
	err = json.Unmarshal(fileData, &note)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return &note, nil
}

// processLabels converts Google Keep labels to Dynalist hashtags
func processLabels(labels []Label) string {
	var hashtags []string
	for _, label := range labels {
		hashtag := strings.ReplaceAll(label.Name, " ", "_") // Replace spaces with underscores
		hashtags = append(hashtags, "#"+hashtag)
	}
	return strings.Join(hashtags, " ")
}

// findAttachmentFile locates an attachment file in the takeout folder
func findAttachmentFile(folderPath string, attachmentPath string) (string, error) {
	attachmentFile := filepath.Join(folderPath, attachmentPath)
	if _, err := os.Stat(attachmentFile); err == nil {
		return attachmentFile, nil
	}
	return "", fmt.Errorf("attachment file not found: %s", attachmentPath)
}

// shortenFilename shortens a filename for use as a title
func shortenFilename(filename string) string {
	name := filepath.Base(filename)
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(name, ext)
	if len(base) > 20 {
		base = base[:20] + "..."
	}
	return base
}
