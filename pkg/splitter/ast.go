package splitter

import (
	"fmt"
	"os"
	"strings"

	"github.com/ranwei/claude-context/pkg"
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
