package tests

import (
	"os"
	"testing"

	"github.com/ranwei/claude-context/pkg/splitter"
)

func TestSplitGoCode(t *testing.T) {
	testFile := "/tmp/test.go"
	code := `package main

// Hello prints a greeting
func Hello() {
	println("Hello, World!")
}

// Add returns the sum of two numbers
func Add(a, b int) int {
	return a + b
}
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "go")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}

	found := false
	for _, chunk := range chunks {
		if contains(chunk.Text, "Hello") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Expected to find 'Hello' function in chunks")
	}
}

func TestSplitPythonCode(t *testing.T) {
	testFile := "/tmp/test.py"
	code := `def hello():
    print("Hello")

def add(a, b):
    return a + b
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "python")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[0:len(substr)] == substr || len(s) > len(substr) && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
