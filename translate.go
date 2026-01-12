package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/translate"
	readingtime "github.com/begmaroman/reading-time"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

type Translation struct {
	DefaultLanguage string   `json:"default_language"`
	Languages       []string `json:"languages"`
	FilePath        string   `json:"file_path"`
	FileNames       []string `json:"file_names"`
	CredentialsPath string   `json:"credentials_path,omitempty"`
	ProjectID       string   `json:"project_id,omitempty"`
}

// Translator holds the translate client, context, and compiled regex patterns
type Translator struct {
	client  *translate.Client
	ctx     context.Context
	regexes struct {
		urlExtract      *regexp.Regexp
		boldFix         *regexp.Regexp
		underlineFix    *regexp.Regexp
		videoShortcode  *regexp.Regexp
		youtubeShortcode *regexp.Regexp
		urlReplace      *regexp.Regexp
	}
	htmlEntities map[string]string
}

// unquoteYAML removes quotes from YAML string values
func unquoteYAML(value string) string {
	// Handle double quotes
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		// Unescape escaped quotes and other characters
		unquoted := value[1 : len(value)-1]
		unquoted = strings.ReplaceAll(unquoted, "\\\"", "\"")
		unquoted = strings.ReplaceAll(unquoted, "\\\\", "\\")
		return unquoted
	}
	// Handle single quotes
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		return value[1 : len(value)-1]
	}
	return value
}

// NewTranslator creates a new Translator instance with initialized client and compiled regex patterns
func NewTranslator(credentialsPath string) (*Translator, error) {
	ctx := context.Background()
	
	// Read the credentials file to extract the project ID
	credData, err := os.ReadFile(credentialsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read credentials file: %w", err)
	}
	
	var credJSON map[string]interface{}
	if err := json.Unmarshal(credData, &credJSON); err != nil {
		return nil, fmt.Errorf("failed to parse credentials JSON: %w", err)
	}
	
	projectID, ok := credJSON["project_id"].(string)
	if !ok || projectID == "" {
		return nil, fmt.Errorf("project_id not found in credentials file")
	}
	
	log.Printf("Using Google Cloud project: %s", projectID)
	
	client, err := translate.NewClient(ctx, option.WithCredentialsFile(credentialsPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create translate client: %w", err)
	}

	t := &Translator{
		client: client,
		ctx:    ctx,
	}

	// Pre-compile all regex patterns
	t.regexes.urlExtract = regexp.MustCompile(`]\([-a-zA-Z0-9@:%._\+~#=\/]{1,256}\)`)
	t.regexes.boldFix = regexp.MustCompile(` (\*\*) ([A-za-z0-9]+) (\*\*)`)
	t.regexes.underlineFix = regexp.MustCompile(` (\*) ([A-za-z0-9]+) (\*)`)
	t.regexes.videoShortcode = regexp.MustCompile(`({{)(<)[ ]{1,3}([vV]ideo)`)
	t.regexes.youtubeShortcode = regexp.MustCompile(`({{)(<)[ ]{1,3}([yY]outube)`)
	t.regexes.urlReplace = regexp.MustCompile(`] \([-a-zA-Z0-9@:%._\+~#=\/ ]{1,256}\)`)

	// HTML entity replacements
	t.htmlEntities = map[string]string{
		"&quot;": "\"",
		"&gt;":   ">",
		"&lt;":   "<",
		"&#39;":  "'",
	}

	return t, nil
}

// Close closes the translate client
func (t *Translator) Close() error {
	return t.client.Close()
}

// translateBatch translates multiple texts at once for efficiency
func (t *Translator) translateBatch(targetLanguage string, texts []string, model string) ([]string, error) {
	if len(texts) == 0 {
		return []string{}, nil
	}

	// Filter out empty and whitespace-only strings before sending to API
	filteredTexts := make([]string, 0, len(texts))
	originalIndices := make([]int, 0, len(texts))
	for i, text := range texts {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			filteredTexts = append(filteredTexts, text)
			originalIndices = append(originalIndices, i)
		}
	}

	if len(filteredTexts) == 0 {
		// Return empty strings for all inputs if all were filtered out
		return make([]string, len(texts)), nil
	}

	lang, err := language.Parse(targetLanguage)
	if err != nil {
		return nil, fmt.Errorf("language.Parse failed for '%s': %w", targetLanguage, err)
	}

	// Process in smaller batches to avoid API limits
	const maxBatchSize = 10
	allResults := make([]string, len(texts))
	
	// Process in chunks
	for batchStart := 0; batchStart < len(filteredTexts); batchStart += maxBatchSize {
		batchEnd := batchStart + maxBatchSize
		if batchEnd > len(filteredTexts) {
			batchEnd = len(filteredTexts)
		}
		
		batchTexts := filteredTexts[batchStart:batchEnd]
		
		log.Printf("Translating batch of %d texts to language '%s' (batch %d of %d)", 
			len(batchTexts), targetLanguage, batchStart/maxBatchSize+1, (len(filteredTexts)+maxBatchSize-1)/maxBatchSize)
		if len(batchTexts) > 0 {
			log.Printf("First text in batch (length %d): %q", len(batchTexts[0]), batchTexts[0])
		}
		
		resp, err := t.client.Translate(t.ctx, batchTexts, lang, &translate.Options{
			Model: model,
		})
		if err != nil {
			log.Printf("Translation failed for language '%s' with %d texts in batch", targetLanguage, len(batchTexts))
			if len(batchTexts) > 0 {
				log.Printf("First text being translated: %q", batchTexts[0])
			}
			return nil, fmt.Errorf("Translate: %w", err)
		}

		if len(resp) != len(batchTexts) {
			return nil, fmt.Errorf("translation response length mismatch: expected %d, got %d", len(batchTexts), len(resp))
		}

		// Map translations back to original positions
		for i, respIdx := range originalIndices[batchStart:batchEnd] {
			if batchStart+i < len(resp) {
				allResults[respIdx] = resp[i].Text
			}
		}
		
		// Add a small delay between batches to avoid rate limiting
		if batchEnd < len(filteredTexts) {
			time.Sleep(100 * time.Millisecond)
		}
	}
	
	return allResults, nil
}

// translateText translates a single text string
func (t *Translator) translateText(targetLanguage, text, model string) (string, error) {
	results, err := t.translateBatch(targetLanguage, []string{text}, model)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", nil
	}
	return results[0], nil
}

// applyPostTranslationFixes applies markdown and URL fixes to translated text
func (t *Translator) applyPostTranslationFixes(originalText, translatedText string) string {
	// Extract URLs before translation because google translate changes [link](http://you.link) to
	// [link] (http://your.link) and it *also* will translate any path
	// components, thus breaking your URLs.
	foundUrls := t.regexes.urlExtract.FindAll([]byte(originalText), -1)

	fixed := translatedText

	// Fix markdown formatting issues
	fixed = string(t.regexes.boldFix.ReplaceAll([]byte(fixed), []byte(" $1$2$3")))

	// Fix HTML entities
	for entity, replacement := range t.htmlEntities {
		fixed = strings.ReplaceAll(fixed, entity, replacement)
	}

	// Fix underline formatting
	fixed = string(t.regexes.underlineFix.ReplaceAll([]byte(fixed), []byte("$1$2$3")))

	// Fix video shortcodes
	fixed = string(t.regexes.videoShortcode.ReplaceAll([]byte(fixed), []byte("$1$2 video")))
	fixed = string(t.regexes.youtubeShortcode.ReplaceAll([]byte(fixed), []byte("$1$2 youtube")))

	// Restore URLs
	for _, foundURL := range foundUrls {
		tmp := t.regexes.urlReplace.FindIndex([]byte(fixed))
		if tmp == nil {
			break
		}
		tBytes := []byte(fixed)
		fixed = fmt.Sprintf("%s(%s%s", string(tBytes[0:tmp[0]+1]), string(foundURL[2:]), string(tBytes[tmp[1]:]))
	}

	return fixed
}

// xl translates text and fixes common markdown/URL issues (for backward compatibility)
func (t *Translator) xl(fromLang string, toLang string, xlate string) (string, error) {
	translated, err := t.translateText(toLang, xlate, "nmt")
	if err != nil {
		return "", err
	}
	return t.applyPostTranslationFixes(xlate, translated), nil
}

// doXlate translates a file from one language to another
func (t *Translator) doXlate(from string, lang string, readFile string, writeFile string) error {
	file, err := os.Open(readFile)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", readFile, err)
	}
	defer file.Close()

	xfile, err := os.Create(writeFile)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", writeFile, err)
	}
	defer xfile.Close()

	var builder strings.Builder
	head := false
	code := false
	scanner := bufio.NewScanner(file)

	// Collect translatable segments for batch translation
	type segment struct {
		index    int
		text     string
		lineType string // "text", "title", "description", "alt"
		original string // original line for non-translatable parts
	}
	var segments []segment
	var segmentIndex int

	for scanner.Scan() {
		ln := scanner.Text()
		if strings.HasPrefix(ln, "{{") {
			segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
			segmentIndex++
			continue
		}
		if strings.HasPrefix(ln, "```") {
			segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
			segmentIndex++
			code = !code
			continue
		}
		if code {
			segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
			segmentIndex++
			continue
		}
		if ln == "---" {
			segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
			segmentIndex++
			head = !head
		} else if !head {
			if strings.HasPrefix(ln, "!") {
				bar := strings.Split(ln, "]")
				if len(bar) > 0 {
					desc := strings.Split(bar[0], "[")
					if len(desc) > 1 {
						segments = append(segments, segment{
							index:    segmentIndex,
							text:     desc[1],
							lineType: "alt",
							original: ln,
						})
						segmentIndex++
					} else {
						segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
						segmentIndex++
					}
				} else {
					segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
					segmentIndex++
				}
			} else if strings.HasPrefix(ln, "> [!") {
				segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
				segmentIndex++
			} else {
				if ln == "" {
					segments = append(segments, segment{index: segmentIndex, original: "\n", lineType: "literal"})
					segmentIndex++
				} else {
					segments = append(segments, segment{
						index:    segmentIndex,
						text:     ln,
						lineType: "text",
						original: ln,
					})
					segmentIndex++
				}
			}
		} else {
			headString := strings.SplitN(ln, ":", 2)
			if len(headString) == 2 {
				if headString[0] == "title" {
					titleValue := strings.TrimSpace(headString[1])
					// Remove quotes from YAML string values
					titleValue = unquoteYAML(titleValue)
					segments = append(segments, segment{
						index:    segmentIndex,
						text:     titleValue,
						lineType: "title",
						original: ln,
					})
					segmentIndex++
				} else if headString[0] == "description" {
					descValue := strings.TrimSpace(headString[1])
					// Remove quotes from YAML string values
					descValue = unquoteYAML(descValue)
					segments = append(segments, segment{
						index:    segmentIndex,
						text:     descValue,
						lineType: "description",
						original: ln,
					})
					segmentIndex++
				} else {
					segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
					segmentIndex++
				}
			} else {
				segments = append(segments, segment{index: segmentIndex, original: ln + "\n", lineType: "literal"})
				segmentIndex++
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	// Batch translate all translatable segments
	textsToTranslate := make([]string, 0)
	translatableIndices := make([]int, 0)
	for i, seg := range segments {
		// Filter out empty strings and whitespace-only strings
		trimmedText := strings.TrimSpace(seg.text)
		if trimmedText != "" {
			textsToTranslate = append(textsToTranslate, seg.text)
			translatableIndices = append(translatableIndices, i)
		}
	}

	var translations []string
	if len(textsToTranslate) > 0 {
		translations, err = t.translateBatch(lang, textsToTranslate, "nmt")
		if err != nil {
			return fmt.Errorf("batch translation failed: %w", err)
		}
		if len(translations) != len(textsToTranslate) {
			return fmt.Errorf("translation count mismatch: expected %d, got %d", len(textsToTranslate), len(translations))
		}
	}

	// Create a map of segment index to translation
	translationMap := make(map[int]string)
	for i, origIdx := range translatableIndices {
		if i < len(translations) {
			translationMap[origIdx] = translations[i]
		}
	}

	// Apply post-translation fixes and build output
	for i, seg := range segments {
		if seg.lineType == "literal" {
			builder.WriteString(seg.original)
		} else if strings.TrimSpace(seg.text) == "" {
			// If segment has no text to translate, write original
			builder.WriteString(seg.original)
			if !strings.HasSuffix(seg.original, "\n") {
				builder.WriteString("\n")
			}
		} else {
			translated, exists := translationMap[i]
			if !exists {
				return fmt.Errorf("no translation found for segment %d", i)
			}

			// Apply post-translation fixes
			fixed := t.applyPostTranslationFixes(seg.text, translated)

			switch seg.lineType {
			case "title":
				builder.WriteString("title: ")
				builder.WriteString(fixed)
				builder.WriteString("\n")
			case "description":
				builder.WriteString("description: ")
				builder.WriteString(fixed)
				builder.WriteString("\n")
			case "alt":
				bar := strings.Split(seg.original, "]")
				if len(bar) > 1 {
					builder.WriteString("![")
					builder.WriteString(fixed)
					builder.WriteString("]")
					builder.WriteString(bar[1])
					builder.WriteString("\n")
				} else {
					builder.WriteString(seg.original)
					builder.WriteString("\n")
				}
			case "text":
				builder.WriteString(fixed)
				builder.WriteString("\n")
			}
		}
	}

	_, err = xfile.WriteString(builder.String())
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// isValueInList checks if a value is in a list
func isValueInList(value string, list []string) bool {
	for _, v := range list {
		if v == value {
			return true
		}
	}
	return false
}

// getFile recursively finds and translates files
func (t *Translator) getFile(from string, path string, lang string, fileNames []string) error {
	thisDir, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", path, err)
	}

	// Create a map for quick lookup of valid file names
	validNames := make(map[string]bool)
	for _, name := range fileNames {
		validNames[name] = true
	}

	for _, f := range thisDir {
		if f.IsDir() {
			if f.Name() == "images" {
				continue
			}
			if err := t.getFile(from, path+"/"+f.Name(), lang, fileNames); err != nil {
				return err
			}
		} else {
			nameParts := strings.Split(f.Name(), ".")
			// Must have at least: [baseName, language, "md"]
			// Format: baseName.language.md (e.g., index.en.md)
			if len(nameParts) < 3 {
				continue
			}

			// Check if file extension is .md (only process markdown files)
			if nameParts[len(nameParts)-1] != "md" {
				continue
			}

			// Extract base name (everything before the language code)
			// For "index.en.md", baseName would be "index"
			baseName := nameParts[0]

			// Check if this base name is in our list of valid file names
			if !validNames[baseName] {
				continue
			}

			// Check if this is a source language file
			fileLang := nameParts[len(nameParts)-2]
			if fileLang != from {
				continue
			}

			fromFile := fmt.Sprintf("%s/%s.%s.md", path, baseName, from)
			toFile := fmt.Sprintf("%s/%s.%s.md", path, baseName, lang)

			_, err := os.Stat(toFile)
			if !os.IsNotExist(err) {
				if baseName != "_index" {
					if err := addReadingTime(fromFile); err != nil {
						log.Printf("Warning: failed to add reading time to %s: %v", fromFile, err)
					}
					if err := addReadingTime(toFile); err != nil {
						log.Printf("Warning: failed to add reading time to %s: %v", toFile, err)
					}
				}
				continue
			}

			if err := addReadingTime(fromFile); err != nil {
				log.Printf("Warning: failed to add reading time to %s: %v", fromFile, err)
			}

			fmt.Printf("Translating:\t %s\nto: \t\t%s\n", fromFile, toFile)
			if err := t.doXlate(from, lang, fromFile, toFile); err != nil {
				return fmt.Errorf("translation failed for %s: %w", fromFile, err)
			}
		}
	}
	return nil
}

// addReadingTime adds reading time to front matter if not present
func addReadingTime(file string) error {
	f, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", file, err)
	}

	content := string(f)
	if strings.Contains(content, "reading_time:") {
		return nil
	}

	estimation := readingtime.Estimate(content)
	fm := strings.LastIndex(content, "---")
	if fm == -1 {
		return fmt.Errorf("no front matter delimiter found in %s", file)
	}

	newArt := content[:fm]
	fw, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", file, err)
	}
	defer fw.Close()

	if _, err := fw.WriteString(newArt); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	mins := int(estimation.Duration.Minutes())
	if mins > 1 {
		if _, err := fw.WriteString(fmt.Sprintf("reading_time: %d minutes\n", mins)); err != nil {
			return fmt.Errorf("failed to write reading time: %w", err)
		}
	} else if mins == 1 {
		if _, err := fw.WriteString(fmt.Sprintf("reading_time: %d minute\n", mins)); err != nil {
			return fmt.Errorf("failed to write reading time: %w", err)
		}
	}

	if _, err := fw.WriteString(content[fm:]); err != nil {
		return fmt.Errorf("failed to write remaining content: %w", err)
	}

	return nil
}

// loadConfig loads configuration from config.json
func loadConfig(configPath string) (*Translation, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Translation
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Set defaults if not provided
	if config.CredentialsPath == "" {
		config.CredentialsPath = "google-secret.json"
	}
	if config.DefaultLanguage == "" {
		config.DefaultLanguage = "en"
	}

	return &config, nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: translate <file_or_directory_path>")
	}

	// Load configuration
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Filter out default language from target languages
	targetLanguages := make([]string, 0)
	for _, lang := range config.Languages {
		if lang != config.DefaultLanguage {
			targetLanguages = append(targetLanguages, lang)
		}
	}

	if len(targetLanguages) == 0 {
		log.Fatal("No target languages configured")
	}

	// Set default file names if not configured
	fileNames := config.FileNames
	if len(fileNames) == 0 {
		fileNames = []string{"index", "_index"}
		log.Printf("No file_names configured, using defaults: %v", fileNames)
	}

	// Create translator instance
	translator, err := NewTranslator(config.CredentialsPath)
	if err != nil {
		log.Fatalf("Failed to initialize translator: %v", err)
	}
	defer translator.Close()

	dir := os.Args[1]
	fi, err := os.Stat(dir)
	if err != nil {
		log.Fatalf("Failed to stat path %s: %v", dir, err)
	}

	// Process languages sequentially to avoid rate limiting
	var errors []error

	for _, lang := range targetLanguages {
		targetLang := lang
		
		var err error
		switch mode := fi.Mode(); {
		case mode.IsDir():
			err = translator.getFile(config.DefaultLanguage, dir, targetLang, fileNames)
		case mode.IsRegular():
			fileName := filepath.Base(dir)
			dirPath := filepath.Dir(dir)
			fn := strings.Split(fileName, ".")
			if len(fn) < 2 {
				err = fmt.Errorf("invalid file name format: %s", dir)
			} else {
				// Only process .md files
				if fn[len(fn)-1] != "md" {
					err = fmt.Errorf("only markdown (.md) files are supported, got: %s", fn[len(fn)-1])
				} else {
					// Extract base name (everything before the language code)
					// For "index.en.md", baseName would be "index"
					baseName := fn[0]
					writeFile := filepath.Join(dirPath, fmt.Sprintf("%s.%s.%s", baseName, targetLang, fn[len(fn)-1]))
					err = translator.doXlate(config.DefaultLanguage, targetLang, dir, writeFile)
				}
			}
		}

		if err != nil {
			errors = append(errors, fmt.Errorf("language %s: %w", targetLang, err))
		}
		
		// Small delay between language translations to avoid API rate limits
		if len(targetLanguages) > 1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if len(errors) > 0 {
		log.Fatalf("Translation errors occurred:\n%v", errors)
	}
}
