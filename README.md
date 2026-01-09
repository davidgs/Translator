# Hugo Page Translator

A high-performance Go program that automatically translates Hugo blog posts and pages to multiple languages using Google Cloud Translate API.

## Features

- **Batch Translation**: Translates multiple text segments in a single API call for efficiency
- **Parallel Processing**: Processes multiple languages concurrently
- **Smart Markdown Handling**: Preserves code blocks, images, URLs, and Hugo shortcodes
- **Post-Translation Fixes**: Automatically fixes common markdown formatting issues introduced by translation
- **Reading Time Calculation**: Automatically adds reading time estimates to front matter
- **Configuration-Based**: Uses `config.json` for easy language and settings management
- **Recursive Directory Scanning**: Automatically finds and translates all `index.en.md` files in a directory tree

## Requirements

- Go 1.15 or later
- Google Cloud account with Translate API enabled
- Google Cloud service account credentials JSON file

## Google Cloud Setup

1. Follow the [Google Cloud Translate API setup guide](https://cloud.google.com/translate/docs/setup)
2. Create a service account and download the credentials JSON file
3. Place the credentials file as `google-secret.json` in the project directory (or configure a custom path in `config.json`)

## Installation

```bash
git clone <repository-url>
cd Translator
go mod download
go build -o translate translate.go
```

## Configuration

Create a `config.json` file in the project root:

```json
{
  "default_language": "en",
  "languages": ["en", "es", "fr", "de", "nl"],
  "credentials_path": "google-secret.json",
  "file_names": ["_index", "index", "about"]
}
```

### Configuration Options

- `default_language`: Source language (default: "en")
- `languages`: Array of all languages including the default
- `credentials_path`: Path to Google Cloud credentials JSON (default: "google-secret.json")
- `file_names`: File name patterns to look for (optional, defaults to "index" and "_index")

## Usage

### Translate a Single File

```bash
./translate /path/to/file.md
```

This will create translated versions for all target languages configured in `config.json` (excluding the default language).

### Translate All Files in a Directory

```bash
./translate /path/to/content/directory
```

The program will recursively scan the directory, find all `index.en.md` and `_index.en.md` files, and translate them to all configured target languages.

### Example Output

```
Translating:	 /content/blog/post/index.en.md
to: 		 /content/blog/post/index.es.md
Translating:	 /content/blog/post/index.en.md
to: 		 /content/blog/post/index.fr.md
Translating:	 /content/blog/post/index.en.md
to: 		 /content/blog/post/index.de.md
```

## How It Works

1. **File Parsing**: The program intelligently parses Markdown files, identifying:
   - Front matter (YAML)
   - Code blocks (preserved as-is)
   - Images (translates alt text only)
   - Hugo shortcodes (preserved)
   - Callouts (preserved)
   - Regular content (translated)

2. **Batch Translation**: All translatable segments are collected and sent to Google Translate API in batches for efficiency

3. **Post-Processing**: After translation, the program:
   - Fixes markdown formatting (bold, italic)
   - Restores URLs that were modified by translation
   - Fixes HTML entities
   - Preserves Hugo shortcodes

4. **Reading Time**: Automatically calculates and adds reading time estimates to front matter

## File Naming Convention

**Important**: Your source files must follow this naming pattern:
- `index.en.md` (not just `index.md`)
- `_index.en.md` for section index files

The program will create translated versions like:
- `index.es.md`
- `index.fr.md`
- `index.de.md`

## Supported Features

- ✅ Front matter translation (title, description)
- ✅ Code block preservation
- ✅ Image alt text translation
- ✅ URL preservation
- ✅ Markdown formatting fixes
- ✅ Hugo shortcode preservation
- ✅ Callout preservation
- ✅ Reading time calculation
- ✅ Recursive directory scanning
- ✅ Parallel language processing

## Performance Optimizations

- **Client Reuse**: Single translate client instance reused across all translations
- **Batch Translation**: Multiple texts translated in single API calls
- **Pre-compiled Regex**: All regex patterns compiled once at startup
- **Parallel Processing**: Multiple languages processed concurrently
- **Efficient String Building**: Uses `strings.Builder` for output construction

## Testing

Run the test suite:

```bash
# Run all tests
go test -v

# Run tests with coverage
go test -cover -v

# Run only unit tests (no API calls)
go test -v -run "TestIsValueInList|TestLoadConfig|TestApplyPostTranslationFixes|TestAddReadingTime"
```

Tests that require Google Translate API credentials will automatically skip if `google-secret.json` is not found.

## Limitations

- Designed for Hugo sites using the Toha theme, but should work with other themes
- Requires Google Cloud Translate API quota
- Some markdown formatting may need manual review after translation
- URLs in translated paths may be modified (program attempts to restore them)

## Contributing

PRs and issues are welcome! Please ensure:
- Code follows Go conventions
- Tests pass (`go test`)
- README is updated for new features

## License

[Add your license here]

## Acknowledgments

Built for Hugo static site generation with Google Cloud Translate API.
