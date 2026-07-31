// Package customization - Group-Based File Extension Management Subsystem
//
// Objectives:
// - Provides OS- and browser-independent file extension validation rules for attachments.
// - Replaces flaky browser MIME detection (file.type) with deterministic extension group aliases (@images, @office, @archives).
// - Supports compound extensions (.tar.gz) and case-insensitive matching across operating systems.
//
// Core Components:
// - DefaultFileGroups: Built-in dictionary of standard file group aliases.
// - LoadCustomFileGroups: Reads external JSON file group maps for admin custom overrides.
// - NormalizeExtension: Sanitizes extensions into lowercase dot-prefixed format (.png, .pdf).
// - ExpandAcceptedFileTypes: Resolves group tokens and MIME tokens into explicit extension lists.
// - IsFilenameAllowed: Validates whether a target filename matches any permitted extension.
package customization

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// DefaultFileGroups defines standard extension groups for easy administration.
var DefaultFileGroups = map[string][]string{
	"@archives":  {".zip", ".7z", ".rar", ".tar", ".gz", ".tgz", ".bz2", ".xz", ".iso", ".cab", ".zst"},
	"@images":    {".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg", ".bmp", ".ico", ".tiff", ".heic"},
	"@video":     {".mp4", ".mkv", ".avi", ".mov", ".wmv", ".webm", ".flv", ".m4v", ".3gp"},
	"@audio":     {".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma", ".opus"},
	"@office":    {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".txt", ".rtf", ".csv"},
	"@documents": {".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".odt", ".ods", ".odp", ".txt", ".rtf", ".csv"},
	"@packages":  {".deb", ".rpm", ".apk", ".msi", ".pkg", ".appimage", ".dmg", ".flatpakref", ".snap", ".ipa"},
	"@binaries":  {".exe", ".bin", ".dll", ".so", ".dylib", ".elf", ".dat"},
	"@code":      {".json", ".xml", ".yaml", ".yml", ".py", ".js", ".ts", ".go", ".sh", ".ps1", ".html", ".css", ".sql"},
}

// LoadCustomFileGroups loads additional or overridden file groups from a JSON file.
func LoadCustomFileGroups(path string) (map[string][]string, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	groups := make(map[string][]string)
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}

	// Normalize group names and extensions
	normalized := make(map[string][]string)
	for groupName, exts := range groups {
		if !strings.HasPrefix(groupName, "@") {
			groupName = "@" + groupName
		}
		groupName = strings.ToLower(groupName)
		var normExts []string
		for _, ext := range exts {
			normExt := NormalizeExtension(ext)
			if normExt != "" {
				normExts = append(normExts, normExt)
			}
		}
		normalized[groupName] = normExts
	}

	return normalized, nil
}

// NormalizeExtension ensures extensions start with a dot and are lowercased.
func NormalizeExtension(ext string) string {
	ext = strings.TrimSpace(strings.ToLower(ext))
	if ext == "" {
		return ""
	}
	ext = strings.TrimPrefix(ext, "*")
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}

// ExpandAcceptedFileTypes takes an acceptedFileTypes string (e.g. "@images, @packages, .pdf, zip")
// and expands all group aliases and individual extensions into a list of normalized extensions.
func ExpandAcceptedFileTypes(input string, customGroups map[string][]string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}

	rawTokens := strings.Split(input, ",")
	seen := make(map[string]bool)
	var result []string

	addExt := func(ext string) {
		norm := NormalizeExtension(ext)
		if norm != "" && !seen[norm] {
			seen[norm] = true
			result = append(result, norm)
		}
	}

	for _, token := range rawTokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}

		if strings.HasPrefix(token, "@") {
			groupKey := strings.ToLower(token)
			// Check custom groups first, then default groups
			exts, ok := customGroups[groupKey]
			if !ok {
				exts, ok = DefaultFileGroups[groupKey]
			}
			if ok {
				for _, ext := range exts {
					addExt(ext)
				}
				continue
			}
		}

		if strings.Contains(token, "/") {
			tokenLower := strings.ToLower(token)
			switch tokenLower {
			case "image/*":
				for _, ext := range DefaultFileGroups["@images"] {
					addExt(ext)
				}
				continue
			case "video/*":
				for _, ext := range DefaultFileGroups["@video"] {
					addExt(ext)
				}
				continue
			case "audio/*":
				for _, ext := range DefaultFileGroups["@audio"] {
					addExt(ext)
				}
				continue
			case "text/*":
				addExt(".txt")
				addExt(".md")
				addExt(".rtf")
				addExt(".csv")
				addExt(".html")
				continue
			case "image/png":
				addExt(".png")
				continue
			case "image/jpeg", "image/jpg":
				addExt(".jpg")
				addExt(".jpeg")
				continue
			case "image/gif":
				addExt(".gif")
				continue
			case "application/pdf":
				addExt(".pdf")
				continue
			case "application/zip":
				addExt(".zip")
				continue
			}
			_, sub, found := strings.Cut(tokenLower, "/")
			if found && sub != "*" && sub != "" {
				addExt(sub)
				continue
			}
		}

		// Individual extension or pattern
		addExt(token)
	}

	return result
}

// IsFilenameAllowed checks if a filename's extension is permitted by the allowed extensions list.
func IsFilenameAllowed(filename string, allowedExts []string) bool {
	if len(allowedExts) == 0 {
		return true // No restriction configured
	}

	lowerName := strings.ToLower(filepath.Base(filename))
	for _, ext := range allowedExts {
		if strings.HasSuffix(lowerName, ext) {
			return true
		}
	}

	return false
}
