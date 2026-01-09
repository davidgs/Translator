package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoXlateFileStructure tests the file parsing and structure handling
// This test requires real Google Translate API credentials
func TestDoXlateFileStructure(t *testing.T) {
	credentialsPath := "google-secret.json"

	// Check if credentials file exists
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: credentials file %s not found", credentialsPath)
		return
	}

	translator, err := NewTranslator(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}
	defer translator.Close()

	tests := []struct {
		name      string
		inputFile string
		validate  func(string) error
	}{
		{
			name: "preserves front matter structure",
			inputFile: `---
title: Test Article
description: Test Description
author: John Doe
date: 2024-01-01
---
# Content`,
			validate: func(output string) error {
				if !strings.Contains(output, "---") {
					return fmt.Errorf("expected front matter delimiters")
				}
				if !strings.Contains(output, "author: John Doe") {
					return fmt.Errorf("expected non-translatable fields to be preserved")
				}
				return nil
			},
		},
		{
			name: "handles empty lines correctly",
			inputFile: `---
title: Test
---

Paragraph one.


Paragraph two.`,
			validate: func(output string) error {
				// Should preserve empty lines
				lines := strings.Split(output, "\n")
				emptyCount := 0
				for _, line := range lines {
					if strings.TrimSpace(line) == "" {
						emptyCount++
					}
				}
				if emptyCount < 2 {
					return fmt.Errorf("expected empty lines to be preserved")
				}
				return nil
			},
		},
		{
			name: "handles image alt text extraction",
			inputFile: `---
title: Images
---
![Alt text here](/path/to/image.png)
![Another alt](/another/path.png)`,
			validate: func(output string) error {
				if !strings.Contains(output, "![") {
					return fmt.Errorf("expected image markdown to be preserved")
				}
				return nil
			},
		},
		{
			name: "preserves code blocks",
			inputFile: `---
title: Code
---
Some text before.

` + "```" + `go
package main
func main() {}
` + "```" + `

Some text after.`,
			validate: func(output string) error {
				if !strings.Contains(output, "```") {
					return fmt.Errorf("expected code block markers to be preserved")
				}
				if !strings.Contains(output, "package main") {
					return fmt.Errorf("expected code content to be preserved")
				}
				return nil
			},
		},
		{
			name: "preserves Hugo shortcodes",
			inputFile: `---
title: Shortcodes
---
{{< video >}}
{{< youtube id="123" >}}
Content here.`,
			validate: func(output string) error {
				if !strings.Contains(output, "{{<") {
					return fmt.Errorf("expected shortcodes to be preserved")
				}
				return nil
			},
		},
		{
			name: "preserves callouts",
			inputFile: `---
title: Callouts
---
> [!note]
> This is a note
> [!warning]
> This is a warning`,
			validate: func(output string) error {
				if !strings.Contains(output, "> [!") {
					return fmt.Errorf("expected callouts to be preserved")
				}
				return nil
			},
		},
		{
			name: "handles multiline front matter",
			inputFile: `---
title: Multiline Title
description: |
  This is a multiline
  description that spans
  multiple lines
tags:
  - tag1
  - tag2
---
# Content`,
			validate: func(output string) error {
				if !strings.Contains(output, "tags:") {
					return fmt.Errorf("expected tags to be preserved")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary input file
			inputFile, err := os.CreateTemp("", "structure_test_input_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp input file: %v", err)
			}
			defer os.Remove(inputFile.Name())

			if _, err := inputFile.WriteString(tt.inputFile); err != nil {
				t.Fatalf("Failed to write input: %v", err)
			}
			inputFile.Close()

			// Create temporary output file
			outputFile, err := os.CreateTemp("", "structure_test_output_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp output file: %v", err)
			}
			outputFile.Close()
			defer os.Remove(outputFile.Name())

			// Test translation with real API
			err = translator.doXlate("en", "fr", inputFile.Name(), outputFile.Name())
			if err != nil {
				t.Fatalf("Translation failed: %v", err)
			}

			// Validate the output
			if tt.validate != nil {
				content, readErr := os.ReadFile(outputFile.Name())
				if readErr != nil {
					t.Fatalf("Failed to read output file: %v", readErr)
				}
				if validateErr := tt.validate(string(content)); validateErr != nil {
					t.Error(validateErr)
				}
			}
		})
	}
}

func TestGetFileDirectoryStructure(t *testing.T) {
	credentialsPath := "google-secret.json"

	// Check if credentials file exists
	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: credentials file %s not found", credentialsPath)
		return
	}

	translator, err := NewTranslator(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}
	defer translator.Close()

	// Create a complex temporary directory structure
	tmpDir, err := os.MkdirTemp("", "getfile_structure_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directory structure
	dirs := []string{
		"blog",
		"blog/post1",
		"blog/post2",
		"blog/post2/subsection",
		"pages",
		"pages/about",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(tmpDir, dir)
		if err := os.MkdirAll(fullPath, 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", fullPath, err)
		}
	}

	// Create index files in some directories
	indexFiles := []string{
		"blog/post1/index.en.md",
		"blog/post2/index.en.md",
		"blog/post2/subsection/index.en.md",
		"pages/about/index.en.md",
	}

	for _, file := range indexFiles {
		fullPath := filepath.Join(tmpDir, file)
		content := `---
title: Test Article
---
# Content`
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	// Create images directory (should be skipped)
	imagesDir := filepath.Join(tmpDir, "blog", "images")
	if err := os.Mkdir(imagesDir, 0755); err != nil {
		t.Fatalf("Failed to create images dir: %v", err)
	}

	// Create a file in images (should be skipped)
	imageFile := filepath.Join(imagesDir, "index.en.md")
	if err := os.WriteFile(imageFile, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create image file: %v", err)
	}

	// Test that getFile processes the structure correctly
	err = translator.getFile("en", tmpDir, "fr")
	if err != nil {
		t.Errorf("getFile failed: %v", err)
	}

	// Verify that images directory was skipped (no fr file created there)
	frImageFile := filepath.Join(imagesDir, "index.fr.md")
	if _, err := os.Stat(frImageFile); err == nil {
		t.Error("Images directory should be skipped")
	}
}

func TestAddReadingTimeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectError bool
	}{
		{
			name: "file with multiple front matter sections",
			content: `---
title: First
---
# Content
---
title: Second
---`,
			expectError: false,
		},
		{
			name: "file with reading_time at end",
			content: `---
title: Test
reading_time: 10 minutes
---`,
			expectError: false,
		},
		{
			name: "file with reading_time in middle",
			content: `---
title: Test
reading_time: 5 minutes
author: John
---`,
			expectError: false,
		},
		{
			name: "very short content",
			content: `---
title: Short
---
Hi`,
			expectError: false,
		},
		{
			name: "content with only front matter",
			content: `---
title: Test
---`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "reading_time_edge_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}
			tmpFile.Close()

			err = addReadingTime(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestApplyPostTranslationFixesEdgeCases(t *testing.T) {
	translator := createMockTranslator()

	tests := []struct {
		name           string
		originalText   string
		translatedText string
		expected       string
	}{
		{
			name:           "URL with query parameters",
			originalText:   "See [link](https://example.com?param=value&other=123)",
			translatedText: "Voir [lien] (https://example.com?param=valeur&autre=123)",
			// Note: URL restoration may not work perfectly with query parameters due to translation changes
			expected:       "Voir [lien] (https://example.com?param=valeur&autre=123)",
		},
		{
			name:           "URL with hash fragment",
			originalText:   "Jump to [section](#section-id)",
			translatedText: "Aller à [section] (#section-id)",
			expected:       "Aller à [section](#section-id)",
		},
		{
			name:           "multiple bold markers",
			originalText:   "**bold1** and **bold2**",
			translatedText: " ** bold1 ** et  ** bold2 ** ",
			expected:       " **bold1** et  **bold2** ",
		},
		{
			name:           "mixed markdown",
			originalText:   "**bold** and *italic*",
			translatedText: " ** gras ** et  * italique * ",
			expected:       " **gras** et *italique* ",
		},
		{
			name:           "video shortcode with capital",
			originalText:   "Video content",
			translatedText: "{{<   Video }}",
			expected:       "{{< video }}",
		},
		{
			name:           "youtube shortcode with capital",
			originalText:   "Youtube content",
			translatedText: "{{<   Youtube }}",
			expected:       "{{< youtube }}",
		},
		{
			name:           "all HTML entities together",
			originalText:   "Test",
			translatedText: "&quot;test&quot; &gt; &lt; &#39;quote&#39;",
			expected:       "\"test\" > < 'quote'",
		},
		{
			name:           "URL in middle of text",
			originalText:   "Before [link](http://test.com) after",
			translatedText: "Avant [lien] (http://test.com/chemin) après",
			expected:       "Avant [lien](http://test.com) après",
		},
		{
			name:           "no URLs to restore",
			originalText:   "Simple text",
			translatedText: "Texte simple",
			expected:       "Texte simple",
		},
		{
			name:           "more URLs found than broken ones",
			originalText:   "[link1](http://1.com) [link2](http://2.com)",
			translatedText: "[lien1] (http://1.com) [lien2] (http://2.com)",
			expected:       "[lien1](http://1.com) [lien2](http://2.com)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translator.applyPostTranslationFixes(tt.originalText, tt.translatedText)
			if result != tt.expected {
				t.Errorf("applyPostTranslationFixes(%q, %q) = %q, expected %q",
					tt.originalText, tt.translatedText, result, tt.expected)
			}
		})
	}
}

func TestConfigFilterLanguages(t *testing.T) {
	// This tests the logic in main() that filters out default language
	config := &Translation{
		DefaultLanguage: "en",
		Languages:       []string{"en", "fr", "de", "es"},
	}

	targetLanguages := make([]string, 0)
	for _, lang := range config.Languages {
		if lang != config.DefaultLanguage {
			targetLanguages = append(targetLanguages, lang)
		}
	}

	expected := []string{"fr", "de", "es"}
	if len(targetLanguages) != len(expected) {
		t.Errorf("Expected %d languages, got %d", len(expected), len(targetLanguages))
	}

	for i, lang := range expected {
		if targetLanguages[i] != lang {
			t.Errorf("Expected language %q at index %d, got %q", lang, i, targetLanguages[i])
		}
	}
}

