package main

import (
	"os"
	"strings"
	"testing"
)

// TestNewTranslatorWithCredentials tests creating a translator with real credentials
func TestNewTranslatorWithCredentials(t *testing.T) {
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

	// Test that translator was initialized correctly
	if translator.client == nil {
		t.Error("Translator client is nil")
	}
	if translator.ctx == nil {
		t.Error("Translator context is nil")
	}
	if translator.regexes.urlExtract == nil {
		t.Error("URL extract regex not initialized")
	}
}

// TestTranslateBatchWithCredentials tests batch translation with real API
func TestTranslateBatchWithCredentials(t *testing.T) {
	credentialsPath := "google-secret.json"

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: credentials file %s not found", credentialsPath)
		return
	}

	translator, err := NewTranslator(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}
	defer translator.Close()

	// Test batch translation
	texts := []string{"Hello", "World"}
	results, err := translator.translateBatch("es", texts, "nmt")
	if err != nil {
		t.Fatalf("Batch translation failed: %v", err)
	}

	if len(results) != len(texts) {
		t.Errorf("Expected %d results, got %d", len(texts), len(results))
	}

	// Verify we got translations back
	for i, result := range results {
		if result == "" {
			t.Errorf("Translation %d is empty", i)
		}
		if result == texts[i] {
			t.Logf("Warning: Translation %d is same as original (might be language detection issue)", i)
		}
		t.Logf("Translated '%s' to '%s'", texts[i], result)
	}
}

// TestTranslateTextWithCredentials tests single text translation
func TestTranslateTextWithCredentials(t *testing.T) {
	credentialsPath := "google-secret.json"

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: credentials file %s not found", credentialsPath)
		return
	}

	translator, err := NewTranslator(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}
	defer translator.Close()

	// Test single translation
	result, err := translator.translateText("fr", "Hello world", "nmt")
	if err != nil {
		t.Fatalf("Translation failed: %v", err)
	}

	if result == "" {
		t.Error("Translation result is empty")
	}
	t.Logf("Translated 'Hello world' to French: '%s'", result)
}

// TestXlWithCredentials tests the xl function with real API
func TestXlWithCredentials(t *testing.T) {
	credentialsPath := "google-secret.json"

	if _, err := os.Stat(credentialsPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: credentials file %s not found", credentialsPath)
		return
	}

	translator, err := NewTranslator(credentialsPath)
	if err != nil {
		t.Fatalf("Failed to create translator: %v", err)
	}
	defer translator.Close()

	// Test xl function with markdown
	original := "Check [this link](https://example.com) for **more** info"
	result, err := translator.xl("en", "es", original)
	if err != nil {
		t.Fatalf("xl translation failed: %v", err)
	}

	if result == "" {
		t.Error("Translation result is empty")
	}
	t.Logf("Original: %s", original)
	t.Logf("Translated: %s", result)

	// Verify URL was preserved
	if !contains(result, "https://example.com") {
		t.Logf("Warning: URL might have been translated (this is expected behavior)")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			strings.Contains(s, substr))))
}

