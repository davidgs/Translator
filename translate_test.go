package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// createMockTranslator creates a translator instance without a real client for testing
func createMockTranslator() *Translator {
	t := &Translator{
		ctx: nil, // Not used in post-translation fixes
	}

	// Pre-compile all regex patterns (same as NewTranslator)
	t.regexes.urlExtract = regexp.MustCompile(`]\([-a-zA-Z0-9@:%._\+~#=\/]{1,256}\)`)
	t.regexes.boldFix = regexp.MustCompile(` (\*\*) ([A-za-z0-9]+) (\*\*)`)
	t.regexes.underlineFix = regexp.MustCompile(` (\*) ([A-za-z0-9]+) (\*)`)
	t.regexes.videoShortcode = regexp.MustCompile(`({{)(<)[ ]{1,3}([vV]ideo)`)
	t.regexes.youtubeShortcode = regexp.MustCompile(`({{)(<)[ ]{1,3}([yY]outube)`)
	t.regexes.urlReplace = regexp.MustCompile(`] \([-a-zA-Z0-9@:%._\+~#=\/ ]{1,256}\)`)

	t.htmlEntities = map[string]string{
		"&quot;": "\"",
		"&gt;":   ">",
		"&lt;":   "<",
		"&#39;":  "'",
	}

	return t
}

func createMockTranslatorWithBatch() *Translator {
	translator := createMockTranslator()
	translator.batchTranslate = func(targetLanguage string, texts []string, model string) ([]string, error) {
		results := make([]string, len(texts))
		for i, text := range texts {
			if strings.TrimSpace(text) == "" {
				results[i] = ""
				continue
			}
			results[i] = fmt.Sprintf("%s:%s", targetLanguage, text)
		}
		return results, nil
	}

	return translator
}

func TestIsValueInList(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		list     []string
		expected bool
	}{
		{
			name:     "value exists in list",
			value:    "en",
			list:     []string{"en", "fr", "de"},
			expected: true,
		},
		{
			name:     "value does not exist in list",
			value:    "es",
			list:     []string{"en", "fr", "de"},
			expected: false,
		},
		{
			name:     "empty list",
			value:    "en",
			list:     []string{},
			expected: false,
		},
		{
			name:     "case sensitive match",
			value:    "EN",
			list:     []string{"en", "fr", "de"},
			expected: false,
		},
		{
			name:     "single item list match",
			value:    "en",
			list:     []string{"en"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValueInList(tt.value, tt.list)
			if result != tt.expected {
				t.Errorf("isValueInList(%q, %v) = %v, expected %v", tt.value, tt.list, result, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name        string
		configJSON  string
		expectError bool
		validate    func(*Translation) error
	}{
		{
			name: "valid config",
			configJSON: `{
				"default_language": "en",
				"languages": ["en", "fr", "de", "es"],
				"file_path": "/test/path",
				"file_names": ["index", "_index"]
			}`,
			expectError: false,
			validate: func(c *Translation) error {
				if c.DefaultLanguage != "en" {
					return fmt.Errorf("expected default_language 'en', got %q", c.DefaultLanguage)
				}
				if len(c.Languages) != 4 {
					return fmt.Errorf("expected 4 languages, got %d", len(c.Languages))
				}
				if c.CredentialsPath != "google-secret.json" {
					return fmt.Errorf("expected default credentials path, got %q", c.CredentialsPath)
				}
				return nil
			},
		},
		{
			name: "config with custom credentials",
			configJSON: `{
				"default_language": "en",
				"languages": ["en", "fr"],
				"credentials_path": "/custom/path.json"
			}`,
			expectError: false,
			validate: func(c *Translation) error {
				if c.CredentialsPath != "/custom/path.json" {
					return fmt.Errorf("expected custom credentials path, got %q", c.CredentialsPath)
				}
				return nil
			},
		},
		{
			name: "config with empty default language",
			configJSON: `{
				"languages": ["en", "fr"]
			}`,
			expectError: false,
			validate: func(c *Translation) error {
				if c.DefaultLanguage != "en" {
					return fmt.Errorf("expected default language 'en', got %q", c.DefaultLanguage)
				}
				return nil
			},
		},
		{
			name:        "invalid JSON",
			configJSON:  `{invalid json}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpFile, err := os.CreateTemp("", "config_test_*.json")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.configJSON); err != nil {
				t.Fatalf("Failed to write config: %v", err)
			}
			tmpFile.Close()

			config, err := loadConfig(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.validate != nil {
				if err := tt.validate(config); err != nil {
					t.Error(err)
				}
			}
		})
	}

	// Test non-existent file
	_, err := loadConfig("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestApplyPostTranslationFixes(t *testing.T) {
	translator := createMockTranslator()

	tests := []struct {
		name           string
		originalText   string
		translatedText string
		expected       string
	}{
		{
			name:           "fix HTML entities",
			originalText:   "Hello world",
			translatedText: "Bonjour &quot;monde&quot; &gt; test &lt; foo &#39;bar&#39;",
			expected:       "Bonjour \"monde\" > test < foo 'bar'",
		},
		{
			name:           "fix bold markdown",
			originalText:   "This is **bold** text",
			translatedText: "Ceci est  ** bold ** texte",
			expected:       "Ceci est  **bold** texte",
		},
		{
			name:           "fix underline markdown",
			originalText:   "This is *italic* text",
			translatedText: "Ceci est  * italic * texte",
			expected:       "Ceci est *italic* texte",
		},
		{
			name:           "fix video shortcode",
			originalText:   "Video content",
			translatedText: "{{<   Video }}",
			expected:       "{{< video }}",
		},
		{
			name:           "fix youtube shortcode",
			originalText:   "Youtube content",
			translatedText: "{{<   Youtube }}",
			expected:       "{{< youtube }}",
		},
		{
			name:           "preserve URLs",
			originalText:   "Check [this link](https://example.com/path)",
			translatedText: "Vérifiez [ce lien] (https://example.com/chemin)",
			expected:       "Vérifiez [ce lien](https://example.com/path)",
		},
		{
			name:           "multiple URLs",
			originalText:   "[link1](http://site1.com) and [link2](http://site2.com)",
			translatedText: "[lien1] (http://site1.com) et [lien2] (http://site2.com)",
			expected:       "[lien1](http://site1.com) et [lien2](http://site2.com)",
		},
		{
			name:           "no changes needed",
			originalText:   "Simple text",
			translatedText: "Texte simple",
			expected:       "Texte simple",
		},
		{
			name:           "empty strings",
			originalText:   "",
			translatedText: "",
			expected:       "",
		},
		{
			name:           "complex markdown with URLs",
			originalText:   "See [documentation](https://docs.example.com) for **more** info",
			translatedText: "Voir [documentation] (https://docs.example.com/chemin) pour  ** plus ** infos",
			expected:       "Voir [documentation](https://docs.example.com) pour  **plus** infos",
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

func TestAddReadingTime(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectError    bool
		shouldAddTime  bool
		validateOutput func(string) error
	}{
		{
			name: "file without reading_time",
			content: `---
title: Test Article
description: A test article
---
# Content
This is some content.`,
			expectError:   false,
			shouldAddTime: true,
			validateOutput: func(output string) error {
				if !strings.Contains(output, "reading_time:") {
					return fmt.Errorf("expected reading_time to be added")
				}
				if !strings.Contains(output, "title: Test Article") {
					return fmt.Errorf("expected front matter to be preserved")
				}
				return nil
			},
		},
		{
			name: "file with existing reading_time",
			content: `---
title: Test Article
reading_time: 5 minutes
---
# Content`,
			expectError:   false,
			shouldAddTime: false,
			validateOutput: func(output string) error {
				if strings.Count(output, "reading_time:") != 1 {
					return fmt.Errorf("expected reading_time to appear only once")
				}
				return nil
			},
		},
		{
			name: "file without front matter",
			content: `# Content
This is some content without front matter.`,
			expectError: true,
		},
		{
			name:        "empty file",
			content:     ``,
			expectError: true,
		},
		{
			name: "file with only front matter",
			content: `---
title: Test
---`,
			expectError:   false,
			shouldAddTime: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "reading_time_test_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write content: %v", err)
			}
			tmpFile.Close()

			originalContent := tt.content
			err = addReadingTime(tmpFile.Name())

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			// Read back the file
			updatedContent, err := os.ReadFile(tmpFile.Name())
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			contentStr := string(updatedContent)
			if tt.shouldAddTime {
				if contentStr == originalContent {
					t.Error("Expected reading_time to be added but content unchanged")
				}
			} else {
				if contentStr != originalContent {
					t.Error("Expected content to remain unchanged")
				}
			}

			if tt.validateOutput != nil {
				if err := tt.validateOutput(contentStr); err != nil {
					t.Error(err)
				}
			}
		})
	}

	// Test non-existent file
	err := addReadingTime("/nonexistent/file.md")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestDoXlate(t *testing.T) {
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

	// Test file parsing and translation logic
	tests := []struct {
		name        string
		inputFile   string
		expectError bool
	}{
		{
			name: "simple markdown file",
			inputFile: `---
title: Test Article
description: A test description
---
# Heading
This is some content.
![alt text](/path/to/image.png)
More content here.`,
			expectError: false,
		},
		{
			name: "file with code blocks",
			inputFile: `---
title: Code Example
---
# Code
` + "```" + `go
func main() {
    fmt.Println("Hello")
}
` + "```" + `
Text after code.`,
			expectError: false,
		},
		{
			name: "file with callouts",
			inputFile: `---
title: Callout Test
---
> [!note]
> This is a note callout
Regular content.`,
			expectError: false,
		},
		{
			name: "file with Hugo shortcodes",
			inputFile: `---
title: Shortcode Test
---
{{< video >}}
Content here.`,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary input file
			inputFile, err := os.CreateTemp("", "input_test_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp input file: %v", err)
			}
			defer os.Remove(inputFile.Name())

			if _, err := inputFile.WriteString(tt.inputFile); err != nil {
				t.Fatalf("Failed to write input: %v", err)
			}
			inputFile.Close()

			// Create temporary output file
			outputFile, err := os.CreateTemp("", "output_test_*.md")
			if err != nil {
				t.Fatalf("Failed to create temp output file: %v", err)
			}
			outputFile.Close()
			defer os.Remove(outputFile.Name())

			err = translator.doXlate("en", "fr", inputFile.Name(), outputFile.Name())
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

	// Test non-existent input file
	err = translator.doXlate("en", "fr", "/nonexistent/input.md", "/tmp/output.md")
	if err == nil {
		t.Error("Expected error for non-existent input file")
	}
}

func TestGetFile(t *testing.T) {
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

	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "getfile_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test directory structure
	testDir := filepath.Join(tmpDir, "test")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test dir: %v", err)
	}

	// Create an index file
	indexFile := filepath.Join(testDir, "index.en.md")
	if err := os.WriteFile(indexFile, []byte("---\ntitle: Test\n---\nContent"), 0644); err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	// Create images directory (should be skipped)
	imagesDir := filepath.Join(testDir, "images")
	if err := os.Mkdir(imagesDir, 0755); err != nil {
		t.Fatalf("Failed to create images dir: %v", err)
	}

	// Test that getFile finds and translates the file
	fileNames := []string{"index", "_index"}
	err = translator.getFile("en", testDir, "fr", fileNames)
	if err != nil {
		t.Errorf("getFile failed: %v", err)
	}

	// Verify that images directory was skipped (no fr file created there)
	frImageFile := filepath.Join(imagesDir, "index.fr.md")
	if _, err := os.Stat(frImageFile); err == nil {
		t.Error("Images directory should be skipped")
	}
}

func TestTranslatorRegexCompilation(t *testing.T) {
	translator := createMockTranslator()

	// Test that all regex patterns are compiled
	if translator.regexes.urlExtract == nil {
		t.Error("urlExtract regex not compiled")
	}
	if translator.regexes.boldFix == nil {
		t.Error("boldFix regex not compiled")
	}
	if translator.regexes.underlineFix == nil {
		t.Error("underlineFix regex not compiled")
	}
	if translator.regexes.videoShortcode == nil {
		t.Error("videoShortcode regex not compiled")
	}
	if translator.regexes.youtubeShortcode == nil {
		t.Error("youtubeShortcode regex not compiled")
	}
	if translator.regexes.urlReplace == nil {
		t.Error("urlReplace regex not compiled")
	}

	// Test that HTML entities map is initialized
	if len(translator.htmlEntities) == 0 {
		t.Error("htmlEntities map not initialized")
	}

	expectedEntities := []string{"&quot;", "&gt;", "&lt;", "&#39;"}
	for _, entity := range expectedEntities {
		if _, exists := translator.htmlEntities[entity]; !exists {
			t.Errorf("HTML entity %q not found in map", entity)
		}
	}
}

func TestConfigDefaults(t *testing.T) {
	// Test that defaults are applied correctly
	configJSON := `{
		"languages": ["en", "fr"]
	}`

	tmpFile, err := os.CreateTemp("", "config_defaults_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configJSON); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	config, err := loadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if config.DefaultLanguage != "en" {
		t.Errorf("Expected default language 'en', got %q", config.DefaultLanguage)
	}

	if config.CredentialsPath != "google-secret.json" {
		t.Errorf("Expected default credentials path 'google-secret.json', got %q", config.CredentialsPath)
	}
}

func TestTranslateJSONFile(t *testing.T) {
	translator := createMockTranslatorWithBatch()

	rootDir := t.TempDir()
	readFile := filepath.Join(rootDir, "data", "en", "sample.json")
	writeFile := filepath.Join(rootDir, "data", "es", "sample.json")
	if err := os.MkdirAll(filepath.Dir(readFile), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	input := `{
  "title": "Hello",
  "nested": {"desc": "World"},
  "count": 2,
  "items": ["One", "Two"],
  "flags": [true, false]
}`
	if err := os.WriteFile(readFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write json file: %v", err)
	}

	if err := translator.translateJSONFile("es", readFile, writeFile); err != nil {
		t.Fatalf("translateJSONFile failed: %v", err)
	}

	data, err := os.ReadFile(writeFile)
	if err != nil {
		t.Fatalf("Failed to read translated json: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Failed to parse translated json: %v", err)
	}

	if payload["title"].(string) != "es:Hello" {
		t.Errorf("Expected title to be translated, got %v", payload["title"])
	}

	nestedRaw, ok := payload["nested"]
	if !ok || nestedRaw == nil {
		t.Fatalf("Expected nested section in TOML output")
	}

	nested, ok := nestedRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected nested to be a map, got %T", nestedRaw)
	}

	if nested["desc"].(string) != "es:World" {
		t.Errorf("Expected nested.desc to be translated, got %v", nested["desc"])
	}

	itemsRaw, ok := payload["items"]
	if !ok || itemsRaw == nil {
		t.Fatalf("Expected items in TOML output")
	}

	switch items := itemsRaw.(type) {
	case []interface{}:
		if items[0].(string) != "es:One" || items[1].(string) != "es:Two" {
			t.Errorf("Expected items to be translated, got %v", items)
		}
	case []string:
		if items[0] != "es:One" || items[1] != "es:Two" {
			t.Errorf("Expected items to be translated, got %v", items)
		}
	default:
		t.Fatalf("Unexpected items type %T", itemsRaw)
	}
}

func TestTranslateYAMLFile(t *testing.T) {
	translator := createMockTranslatorWithBatch()

	rootDir := t.TempDir()
	readFile := filepath.Join(rootDir, "data", "en", "sample.yaml")
	writeFile := filepath.Join(rootDir, "data", "es", "sample.yaml")
	if err := os.MkdirAll(filepath.Dir(readFile), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	input := `title: Hello
nested:
  desc: World
items:
  - One
  - Two
flags:
  - true
  - false
`
	if err := os.WriteFile(readFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write yaml file: %v", err)
	}

	if err := translator.translateYAMLFile("es", readFile, writeFile); err != nil {
		t.Fatalf("translateYAMLFile failed: %v", err)
	}

	data, err := os.ReadFile(writeFile)
	if err != nil {
		t.Fatalf("Failed to read translated yaml: %v", err)
	}

	var payload map[string]interface{}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Failed to parse translated yaml: %v", err)
	}

	if payload["title"].(string) != "es:Hello" {
		t.Errorf("Expected title to be translated, got %v", payload["title"])
	}

	nested := payload["nested"].(map[string]interface{})
	if nested["desc"].(string) != "es:World" {
		t.Errorf("Expected nested.desc to be translated, got %v", nested["desc"])
	}

	items := payload["items"].([]interface{})
	if items[0].(string) != "es:One" || items[1].(string) != "es:Two" {
		t.Errorf("Expected items to be translated, got %v", items)
	}
}

func TestTranslateTOMLFile(t *testing.T) {
	translator := createMockTranslatorWithBatch()

	rootDir := t.TempDir()
	readFile := filepath.Join(rootDir, "data", "en", "sample.toml")
	writeFile := filepath.Join(rootDir, "data", "es", "sample.toml")
	if err := os.MkdirAll(filepath.Dir(readFile), 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	input := `title = "Hello"
items = ["One", "Two"]

[nested]
desc = "World"
`
	if err := os.WriteFile(readFile, []byte(input), 0644); err != nil {
		t.Fatalf("Failed to write toml file: %v", err)
	}

	if err := translator.translateTOMLFile("es", readFile, writeFile); err != nil {
		t.Fatalf("translateTOMLFile failed: %v", err)
	}

	data, err := os.ReadFile(writeFile)
	if err != nil {
		t.Fatalf("Failed to read translated toml: %v", err)
	}

	var payload map[string]interface{}
	if err := toml.Unmarshal(data, &payload); err != nil {
		t.Fatalf("Failed to parse translated toml: %v", err)
	}

	if payload["title"].(string) != "es:Hello" {
		t.Errorf("Expected title to be translated, got %v", payload["title"])
	}

	nested := payload["nested"].(map[string]interface{})
	if nested["desc"].(string) != "es:World" {
		t.Errorf("Expected nested.desc to be translated, got %v", nested["desc"])
	}

	items := payload["items"].([]interface{})
	if items[0].(string) != "es:One" || items[1].(string) != "es:Two" {
		t.Errorf("Expected items to be translated, got %v", items)
	}
}

func TestGetDataFiles(t *testing.T) {
	translator := createMockTranslatorWithBatch()

	rootDir := t.TempDir()
	dataDir := filepath.Join(rootDir, "data")
	if err := os.MkdirAll(filepath.Join(dataDir, "en", "nested"), 0755); err != nil {
		t.Fatalf("Failed to create data directories: %v", err)
	}

	jsonFile := filepath.Join(dataDir, "en", "sample.json")
	yamlFile := filepath.Join(dataDir, "en", "nested", "sample.yaml")
	tomlFile := filepath.Join(dataDir, "en", "sample.toml")
	mdFile := filepath.Join(dataDir, "en", "note.md")

	if err := os.WriteFile(jsonFile, []byte(`{"title":"Hello"}`), 0644); err != nil {
		t.Fatalf("Failed to write json file: %v", err)
	}
	if err := os.WriteFile(yamlFile, []byte("title: Hello\n"), 0644); err != nil {
		t.Fatalf("Failed to write yaml file: %v", err)
	}
	if err := os.WriteFile(tomlFile, []byte("title = \"Hello\"\n"), 0644); err != nil {
		t.Fatalf("Failed to write toml file: %v", err)
	}
	if err := os.WriteFile(mdFile, []byte("Hello data file"), 0644); err != nil {
		t.Fatalf("Failed to write markdown file: %v", err)
	}

	if err := translator.getDataFiles("en", dataDir, "es"); err != nil {
		t.Fatalf("getDataFiles failed: %v", err)
	}

	translatedJSON := filepath.Join(dataDir, "es", "sample.json")
	translatedYAML := filepath.Join(dataDir, "es", "nested", "sample.yaml")
	translatedTOML := filepath.Join(dataDir, "es", "sample.toml")
	translatedMD := filepath.Join(dataDir, "es", "note.md")

	for _, file := range []string{translatedJSON, translatedYAML, translatedTOML, translatedMD} {
		if _, err := os.Stat(file); err != nil {
			t.Fatalf("Expected translated file %s to exist: %v", file, err)
		}
	}

	mdContent, err := os.ReadFile(translatedMD)
	if err != nil {
		t.Fatalf("Failed to read translated markdown: %v", err)
	}
	if !strings.Contains(string(mdContent), "es:Hello data file") {
		t.Errorf("Expected markdown to be translated, got %q", string(mdContent))
	}
}

func BenchmarkApplyPostTranslationFixes(b *testing.B) {
	translator := createMockTranslator()
	original := "Check [this link](https://example.com/path) for **more** info"
	translated := "Vérifiez [ce lien] (https://example.com/chemin) pour ( ** ) ( plus ) ( ** ) infos"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		translator.applyPostTranslationFixes(original, translated)
	}
}

func BenchmarkIsValueInList(b *testing.B) {
	list := []string{"en", "fr", "de", "es", "nl", "it", "pt", "ru", "zh", "ja"}
	value := "es"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isValueInList(value, list)
	}
}
