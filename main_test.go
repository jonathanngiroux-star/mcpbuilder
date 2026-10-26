package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMCPBuilderConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	mb, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	if len(mb.servers) == 0 {
		t.Error("Expected default server to be loaded")
	}
}

func TestCreateServer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	mb, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	tools := []Tool{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"input": map[string]interface{}{"type": "string"},
				},
				"required": []string{"input"},
			},
		},
	}
	server := mb.CreateServer("Test Server", "A test server", "stdio", tools)

	if server.Name != "Test Server" {
		t.Errorf("Expected name 'Test Server', got '%s'", server.Name)
	}
	if server.Transport != "stdio" {
		t.Errorf("Expected transport 'stdio', got '%s'", server.Transport)
	}
	if len(server.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(server.Tools))
	}
	if server.ID == "" {
		t.Error("Server should have ID")
	}

	// Check files were generated
	if _, err := os.Stat(filepath.Join(server.WorkingDir, "go.mod")); os.IsNotExist(err) {
		t.Error("go.mod should have been generated")
	}
	if _, err := os.Stat(filepath.Join(server.WorkingDir, "main.go")); os.IsNotExist(err) {
		t.Error("main.go should have been generated")
	}
	if _, err := os.Stat(filepath.Join(server.WorkingDir, "README.md")); os.IsNotExist(err) {
		t.Error("README.md should have been generated")
	}
}

func TestRemoveServer(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	mb, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	server := mb.CreateServer("Test Server", "Test", "stdio", nil)
	if err := mb.RemoveServer(server.ID); err != nil {
		t.Fatalf("Failed to remove server: %v", err)
	}

	_, ok := mb.GetServer(server.ID)
	if ok {
		t.Error("Server should have been removed")
	}
}

func TestListServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	mb, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	mb.CreateServer("Server 1", "Desc 1", "stdio", nil)
	mb.CreateServer("Server 2", "Desc 2", "sse", nil)

	servers := mb.ListServers()
	if len(servers) < 2 {
		t.Errorf("Expected at least 2 servers, got %d", len(servers))
	}
}

func TestServerStatus(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	mb, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	server := mb.CreateServer("Status Test", "Test", "stdio", nil)
	if server.Status != "created" {
		t.Errorf("Expected initial status 'created', got '%s'", server.Status)
	}
}

func TestConfigFileCreation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	_, err := NewMCPBuilder(configPath)
	if err != nil {
		t.Fatalf("Failed to create MCPBuilder: %v", err)
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}

	data, _ := os.ReadFile(configPath)
	if len(data) == 0 {
		t.Error("Config file should not be empty")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled")
	}
}