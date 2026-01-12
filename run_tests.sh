#!/bin/bash
set -e

echo "Running unit tests..."
go test -v -run "TestIsValueInList|TestLoadConfig|TestApplyPostTranslationFixes|TestAddReadingTime|TestTranslatorRegexCompilation|TestConfigDefaults|TestApplyPostTranslationFixesEdgeCases|TestConfigFilterLanguages|TestAddReadingTimeEdgeCases" 2>&1

echo ""
echo "Running integration tests (these require Google Translate API)..."
go test -v -run "TestDoXlateFileStructure|TestGetFileDirectoryStructure" -timeout 60s 2>&1 || echo "Integration tests may require actual API calls"

echo ""
echo "Running all tests with coverage..."
go test -cover -v 2>&1



