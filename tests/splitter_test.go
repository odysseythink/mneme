package tests

import (
	"os"
	"testing"

	"github.com/ranwei/mneme/pkg/splitter"
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

func TestSplitJavaCode(t *testing.T) {
	testFile := "/tmp/test.java"
	code := `public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }

    private int multiply(int x, int y) {
        int result = x * y;
        return result;
    }
}
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "java")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}

	foundAdd := false
	foundMultiply := false
	for _, chunk := range chunks {
		if containsSubstr(chunk.Text, "add") {
			foundAdd = true
		}
		if containsSubstr(chunk.Text, "multiply") {
			foundMultiply = true
		}
	}
	if !foundAdd || !foundMultiply {
		t.Fatalf("Expected to find both 'add' and 'multiply' methods, foundAdd=%v, foundMultiply=%v", foundAdd, foundMultiply)
	}
}

func TestSplitRustCode(t *testing.T) {
	testFile := "/tmp/test.rs"
	code := `pub fn add(a: i32, b: i32) -> i32 {
    a + b
}

fn multiply(x: i32, y: i32) -> i32 {
    let result = x * y;
    result
}

pub async fn fetch_data(url: &str) -> Result<String, Error> {
    let response = make_request(url).await?;
    Ok(response)
}
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "rust")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 3 {
		t.Fatalf("Expected at least 3 chunks, got %d", len(chunks))
	}

	foundAdd := false
	foundMultiply := false
	foundFetch := false
	for _, chunk := range chunks {
		if containsSubstr(chunk.Text, "add") {
			foundAdd = true
		}
		if containsSubstr(chunk.Text, "multiply") {
			foundMultiply = true
		}
		if containsSubstr(chunk.Text, "fetch_data") {
			foundFetch = true
		}
	}
	if !foundAdd || !foundMultiply || !foundFetch {
		t.Fatalf("Expected to find 'add', 'multiply', and 'fetch_data' functions, foundAdd=%v, foundMultiply=%v, foundFetch=%v", foundAdd, foundMultiply, foundFetch)
	}
}

func TestSplitCppCode(t *testing.T) {
	testFile := "/tmp/test.cpp"
	code := `int add(int a, int b) {
    return a + b;
}

std::string greet(const std::string& name) {
    return "Hello, " + name;
}

void Calculator::multiply(int x, int y) {
    result_ = x * y;
}
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "cpp")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 3 {
		t.Fatalf("Expected at least 3 chunks, got %d", len(chunks))
	}

	foundAdd := false
	foundGreet := false
	foundMultiply := false
	for _, chunk := range chunks {
		if containsSubstr(chunk.Text, "add") {
			foundAdd = true
		}
		if containsSubstr(chunk.Text, "greet") {
			foundGreet = true
		}
		if containsSubstr(chunk.Text, "multiply") {
			foundMultiply = true
		}
	}
	if !foundAdd || !foundGreet || !foundMultiply {
		t.Fatalf("Expected to find 'add', 'greet', and 'multiply' functions, foundAdd=%v, foundGreet=%v, foundMultiply=%v", foundAdd, foundGreet, foundMultiply)
	}
}

func TestSplitCSharpCode(t *testing.T) {
	testFile := "/tmp/test.cs"
	code := `public class MathHelper {
    public int Add(int a, int b) {
        return a + b;
    }

    private string Format(int value) {
        return value.ToString();
    }

    public static bool IsPositive(int n) {
        return n > 0;
    }
}
`

	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}
	defer os.Remove(testFile)

	s := splitter.NewSplitter()
	chunks, err := s.Split(testFile, "csharp")

	if err != nil {
		t.Fatalf("Split failed: %v", err)
	}

	if len(chunks) < 3 {
		t.Fatalf("Expected at least 3 chunks, got %d", len(chunks))
	}

	foundAdd := false
	foundFormat := false
	foundIsPositive := false
	for _, chunk := range chunks {
		if containsSubstr(chunk.Text, "Add") {
			foundAdd = true
		}
		if containsSubstr(chunk.Text, "Format") {
			foundFormat = true
		}
		if containsSubstr(chunk.Text, "IsPositive") {
			foundIsPositive = true
		}
	}
	if !foundAdd || !foundFormat || !foundIsPositive {
		t.Fatalf("Expected to find 'Add', 'Format', and 'IsPositive' methods, foundAdd=%v, foundFormat=%v, foundIsPositive=%v", foundAdd, foundFormat, foundIsPositive)
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
