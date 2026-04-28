package splitter

import (
	"fmt"
	"os"
	"strings"

	"github.com/ranwei/mneme/pkg"
)

type ASTSplitter struct{}

func NewSplitter() *ASTSplitter {
	return &ASTSplitter{}
}

func (s *ASTSplitter) Split(filePath string, language string) ([]pkg.Vector, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var chunks []pkg.Vector

	switch language {
	case "go":
		chunks = s.splitGo(filePath, string(content))
	case "python":
		chunks = s.splitPython(filePath, string(content))
	case "js", "ts":
		chunks = s.splitJavaScript(filePath, string(content))
	case "java":
		chunks = s.splitJava(filePath, string(content))
	case "rust":
		chunks = s.splitRust(filePath, string(content))
	case "cpp":
		chunks = s.splitCpp(filePath, string(content))
	case "csharp":
		chunks = s.splitCSharp(filePath, string(content))
	default:
		chunks = s.splitByLines(filePath, string(content), language)
	}

	return chunks, nil
}

func (s *ASTSplitter) splitGo(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inFunc := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc && strings.HasPrefix(trimmed, "func ") {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "go",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inFunc = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inFunc {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "go",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inFunc = false
			}
		} else if !inFunc && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "go",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitPython(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inFunc := false
	indentLevel := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if !inFunc && strings.HasPrefix(trimmed, "def ") {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "python",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inFunc = true
			indentLevel = len(line) - len(strings.TrimLeft(line, " \t"))
		} else if inFunc {
			if len(trimmed) > 0 {
				currentIndent := len(line) - len(strings.TrimLeft(line, " \t"))
				if currentIndent <= indentLevel && !strings.HasPrefix(trimmed, "#") {
					chunks = append(chunks, pkg.Vector{
						FilePath:  filePath,
						Language:  "python",
						StartLine: startLine + 1,
						EndLine:   i,
						Text:      strings.Join(currentChunk, "\n"),
					})
					currentChunk = []string{line}
					startLine = i
					if strings.HasPrefix(trimmed, "def ") {
						indentLevel = currentIndent
					} else {
						inFunc = false
					}
					continue
				}
			}
			currentChunk = append(currentChunk, line)
		} else if !inFunc && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "python",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitJavaScript(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inFunc := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		isFuncStart := strings.HasPrefix(trimmed, "function ") ||
			strings.HasPrefix(trimmed, "const ") ||
			strings.HasPrefix(trimmed, "let ") ||
			strings.HasPrefix(trimmed, "var ") ||
			strings.HasPrefix(trimmed, "async ") ||
			strings.HasPrefix(trimmed, "export ")

		if !inFunc && isFuncStart {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "js",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inFunc = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inFunc {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "js",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inFunc = false
			}
		} else if !inFunc && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "js",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitByLines(filePath string, content string, language string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	chunks := []pkg.Vector{}

	for i := 0; i < len(lines); i += 50 {
		endIdx := i + 50
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		chunk := pkg.Vector{
			FilePath:  filePath,
			Language:  language,
			StartLine: i + 1,
			EndLine:   endIdx,
			Text:      strings.Join(lines[i:endIdx], "\n"),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

func (s *ASTSplitter) splitJava(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inMethod := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for method declarations: contains ( and ) with access modifiers or return type
		isMethodStart := false
		if !inMethod && strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
			// Check for method declaration patterns
			if strings.HasPrefix(trimmed, "public ") ||
				strings.HasPrefix(trimmed, "private ") ||
				strings.HasPrefix(trimmed, "protected ") ||
				strings.HasPrefix(trimmed, "static ") {
				// Avoid matching class declarations and constructors are methods
				if !strings.HasPrefix(trimmed, "public class ") &&
					!strings.HasPrefix(trimmed, "private class ") &&
					!strings.HasPrefix(trimmed, "protected class ") {
					isMethodStart = true
				}
			}
		}

		if isMethodStart {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "java",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inMethod = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inMethod {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "java",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inMethod = false
			}
		} else if !inMethod && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "java",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitRust(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inFunc := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for function declarations: starts with fn, pub fn, pub(crate) fn, async fn, pub async fn
		isFuncStart := false
		if !inFunc {
			if strings.HasPrefix(trimmed, "fn ") ||
				strings.HasPrefix(trimmed, "pub fn ") ||
				strings.HasPrefix(trimmed, "pub(crate) fn ") ||
				strings.HasPrefix(trimmed, "async fn ") ||
				strings.HasPrefix(trimmed, "pub async fn ") ||
				strings.HasPrefix(trimmed, "unsafe fn ") ||
				strings.HasPrefix(trimmed, "pub unsafe fn ") {
				isFuncStart = true
			}
		}

		if isFuncStart {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "rust",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inFunc = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inFunc {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "rust",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inFunc = false
			}
		} else if !inFunc && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "rust",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitCpp(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inFunc := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for function definitions: contains ( and ) and followed by { or has { on same line
		// Skip control structures like if, for, while, switch
		isFuncStart := false
		if !inFunc && strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
			// Check if this looks like a function definition (not a control structure)
			if !strings.HasPrefix(trimmed, "if") &&
				!strings.HasPrefix(trimmed, "for") &&
				!strings.HasPrefix(trimmed, "while") &&
				!strings.HasPrefix(trimmed, "switch") &&
				!strings.HasPrefix(trimmed, "catch") &&
				!strings.Contains(trimmed, "?") { // ternary operator
				isFuncStart = true
			}
		}

		if isFuncStart {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "cpp",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inFunc = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inFunc {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "cpp",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inFunc = false
			}
		} else if !inFunc && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "cpp",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}

func (s *ASTSplitter) splitCSharp(filePath string, content string) []pkg.Vector {
	lines := strings.Split(content, "\n")
	var chunks []pkg.Vector

	var currentChunk []string
	var startLine int
	inMethod := false
	braceCount := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Look for method declarations: contains ( and ) with access modifiers or return type
		isMethodStart := false
		if !inMethod && strings.Contains(trimmed, "(") && strings.Contains(trimmed, ")") {
			// Check for method declaration patterns
			if strings.HasPrefix(trimmed, "public ") ||
				strings.HasPrefix(trimmed, "private ") ||
				strings.HasPrefix(trimmed, "protected ") ||
				strings.HasPrefix(trimmed, "internal ") ||
				strings.HasPrefix(trimmed, "static ") {
				// Avoid matching class declarations
				if !strings.HasPrefix(trimmed, "public class ") &&
					!strings.HasPrefix(trimmed, "private class ") &&
					!strings.HasPrefix(trimmed, "protected class ") &&
					!strings.HasPrefix(trimmed, "internal class ") {
					isMethodStart = true
				}
			}
		}

		if isMethodStart {
			if len(currentChunk) > 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "csharp",
					StartLine: startLine + 1,
					EndLine:   i,
					Text:      strings.Join(currentChunk, "\n"),
				})
			}
			currentChunk = []string{line}
			startLine = i
			inMethod = true
			braceCount = strings.Count(line, "{") - strings.Count(line, "}")
		} else if inMethod {
			currentChunk = append(currentChunk, line)
			braceCount += strings.Count(line, "{") - strings.Count(line, "}")
			if braceCount == 0 {
				chunks = append(chunks, pkg.Vector{
					FilePath:  filePath,
					Language:  "csharp",
					StartLine: startLine + 1,
					EndLine:   i + 1,
					Text:      strings.Join(currentChunk, "\n"),
				})
				currentChunk = nil
				inMethod = false
			}
		} else if !inMethod && len(trimmed) > 0 {
			if len(currentChunk) == 0 {
				startLine = i
			}
			currentChunk = append(currentChunk, line)
		}
	}

	if len(currentChunk) > 0 {
		chunks = append(chunks, pkg.Vector{
			FilePath:  filePath,
			Language:  "csharp",
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentChunk, "\n"),
		})
	}

	return chunks
}
