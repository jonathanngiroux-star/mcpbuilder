package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type MCPServer struct {
	ID          string            `json:"id" yaml:"id"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Version     string            `json:"version" yaml:"version"`
	Transport   string            `json:"transport" yaml:"transport"` // stdio, sse, websocket
	Command     string            `json:"command" yaml:"command"`
	Args        []string          `json:"args" yaml:"args"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	WorkingDir  string            `json:"working_dir" yaml:"working_dir"`
	Tools       []Tool            `json:"tools" yaml:"tools"`
	Resources   []Resource        `json:"resources,omitempty" yaml:"resources,omitempty"`
	Prompts     []Prompt          `json:"prompts,omitempty" yaml:"prompts,omitempty"`
	Status      string            `json:"status" yaml:"status"` // created, running, stopped, error
	PID         int               `json:"pid,omitempty" yaml:"pid,omitempty"`
	CreatedAt   time.Time         `json:"created_at" yaml:"created_at"`
	StartedAt   time.Time         `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

type Tool struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	InputSchema map[string]interface{} `json:"input_schema" yaml:"input_schema"`
	Handler     string                 `json:"handler,omitempty" yaml:"handler,omitempty"` // Go function name
}

type Resource struct {
	URI         string `json:"uri" yaml:"uri"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	MimeType    string `json:"mime_type" yaml:"mime_type"`
}

type Prompt struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	Arguments   []Argument `json:"arguments,omitempty" yaml:"arguments,omitempty"`
}

type Argument struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`
}

type Config struct {
	Servers     []MCPServer `json:"servers" yaml:"servers"`
	TemplatesDir string     `json:"templates_dir,omitempty" yaml:"templates_dir,omitempty"`
	OutputDir   string     `json:"output_dir,omitempty" yaml:"output_dir,omitempty"`
	HermesPath  string     `json:"hermes_path,omitempty" yaml:"hermes_path,omitempty"`
}

type MCPBuilder struct {
	config   *Config
	servers  map[string]*MCPServer
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	processes map[string]*exec.Cmd
	wg       sync.WaitGroup
}

func NewMCPBuilder(configPath string) (*MCPBuilder, error) {
	ctx, cancel := context.WithCancel(context.Background())
	mb := &MCPBuilder{
		config:    &Config{},
		servers:   make(map[string]*MCPServer),
		ctx:       ctx,
		cancel:    cancel,
		processes: make(map[string]*exec.Cmd),
	}

	if err := mb.LoadConfig(configPath); err != nil {
		return nil, err
	}

	for _, s := range mb.config.Servers {
		mb.servers[s.ID] = &s
	}

	if mb.config.TemplatesDir == "" {
		mb.config.TemplatesDir = "./templates"
	}
	if mb.config.OutputDir == "" {
		mb.config.OutputDir = "./servers"
	}
	if mb.config.HermesPath == "" {
		mb.config.HermesPath = "hermes"
	}

	os.MkdirAll(mb.config.TemplatesDir, 0755)
	os.MkdirAll(mb.config.OutputDir, 0755)

	return mb, nil
}

func (mb *MCPBuilder) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return mb.CreateDefaultConfig(path)
		}
		return err
	}

	if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
		return yaml.Unmarshal(data, mb.config)
	}
	return json.Unmarshal(data, mb.config)
}

func (mb *MCPBuilder) CreateDefaultConfig(path string) error {
	mb.config = &Config{
		Servers: []MCPServer{
			{
				ID:          "example-echo",
				Name:        "Echo Server",
				Description: "Simple echo tool for testing",
				Version:     "1.0.0",
				Transport:   "stdio",
				Command:     "go",
				Args:        []string{"run", "echo_server.go"},
				WorkingDir:  "./servers/echo",
				Tools: []Tool{
					{
						Name:        "echo",
						Description: "Echo back the input",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"message": map[string]interface{}{"type": "string"},
							},
							"required": []string{"message"},
						},
						Handler: "EchoHandler",
					},
				},
				Status: "created",
			},
		},
		TemplatesDir: "./templates",
		OutputDir:    "./servers",
		HermesPath:   "hermes",
	}

	os.MkdirAll(filepath.Dir(path), 0755)
	data, _ := yaml.Marshal(mb.config)
	return os.WriteFile(path, data, 0644)
}

func (mb *MCPBuilder) SaveConfig() error {
	mb.mu.RLock()
	mb.config.Servers = make([]MCPServer, 0, len(mb.servers))
	for _, s := range mb.servers {
		mb.config.Servers = append(mb.config.Servers, *s)
	}
	data, _ := yaml.Marshal(mb.config)
	mb.mu.RUnlock()
	return os.WriteFile(mb.configPath(), data, 0644)
}

func (mb *MCPBuilder) configPath() string {
	return "./config.yaml"
}

func (mb *MCPBuilder) CreateServer(name, description, transport string, tools []Tool) *MCPServer {
	id := uuid.New().String()
	server := &MCPServer{
		ID:          id,
		Name:        name,
		Description: description,
		Version:     "1.0.0",
		Transport:   transport,
		Command:     "go",
		Args:        []string{"run", "main.go"},
		WorkingDir:  filepath.Join(mb.config.OutputDir, strings.ToLower(strings.ReplaceAll(name, " ", "-"))),
		Tools:       tools,
		Status:      "created",
		CreatedAt:   time.Now(),
		Metadata:    make(map[string]string),
	}

	mb.mu.Lock()
	mb.servers[id] = server
	mb.config.Servers = append(mb.config.Servers, *server)
	mb.mu.Unlock()

	// Create server directory and files
	os.MkdirAll(server.WorkingDir, 0755)
	mb.generateServerFiles(server)

	mb.SaveConfig()
	return server
}

func (mb *MCPBuilder) generateServerFiles(server *MCPServer) {
	// Generate go.mod
	goMod := fmt.Sprintf(`module %s

go 1.21

require (
	github.com/mark3labs/mcp-go v0.15.0
)
`, filepath.Base(server.WorkingDir))
	os.WriteFile(filepath.Join(server.WorkingDir, "go.mod"), []byte(goMod), 0644)

	// Generate main.go
	toolsCode := ""
	for _, tool := range server.Tools {
		handlerName := tool.Handler
		if handlerName == "" {
			handlerName = strings.Title(strings.ReplaceAll(tool.Name, "-", "")) + "Handler"
		}
		toolsCode += fmt.Sprintf(`
	mcp.AddTool(&mcp.Tool{
		Name:        "%s",
		Description: "%s",
		InputSchema: %s,
		Handler:     %s,
	})
`, tool.Name, tool.Description, jsonMarshal(tool.InputSchema), handlerName)
	}

	mainGo := fmt.Sprintf(`package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

%s

func main() {
	s := server.NewMCPServer("%s", "%s", server.WithToolCapabilities(true))
%s
	if err := s.Run(context.Background(), server.WithStdio()); err != nil {
		log.Fatal(err)
	}
}
`, toolsCode, server.Name, server.Version, toolsCode)

	os.WriteFile(filepath.Join(server.WorkingDir, "main.go"), []byte(mainGo), 0644)

	// Generate handler stubs
	for _, tool := range server.Tools {
		handlerName := tool.Handler
		if handlerName == "" {
			handlerName = strings.Title(strings.ReplaceAll(tool.Name, "-", "")) + "Handler"
		}
		
		handlerGo := fmt.Sprintf(`package main

		import (
			"context"
			"encoding/json"
			"github.com/mark3labs/mcp-go/mcp"
		)

		func %s(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			var args struct {
				Message string ` + "`" + `json:"message"` + "`" + `
			}
			if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return mcp.NewToolResultText("Echo: " + args.Message), nil
		}
		`, handlerName)
		
		handlerFile := strings.ToLower(handlerName) + ".go"
		os.WriteFile(filepath.Join(server.WorkingDir, handlerFile), []byte(handlerGo), 0644)
	}

	// Generate README
	readme := fmt.Sprintf(`# %s

%s

## Tools

`, server.Name, server.Description)
	for _, tool := range server.Tools {
		readme += fmt.Sprintf("- **%s**: %s\n", tool.Name, tool.Description)
	}
	readme += fmt.Sprintf(`
## Running

`+"```bash"+`
cd %s
go mod tidy
go run .
`+"```"+`

## Transport

%s (configured)
`, server.WorkingDir, strings.ToUpper(server.Transport))

	os.WriteFile(filepath.Join(server.WorkingDir, "README.md"), []byte(readme), 0644)
}

func jsonMarshal(v interface{}) string {
	data, _ := json.MarshalIndent(v, "", "  ")
	return string(data)
}

func (mb *MCPBuilder) RemoveServer(id string) error {
	mb.mu.Lock()
	server, ok := mb.servers[id]
	mb.mu.Unlock()
	if !ok {
		return fmt.Errorf("server %s not found", id)
	}

	if server.Status == "running" {
		mb.StopServer(id)
	}

	mb.mu.Lock()
	delete(mb.servers, id)
	mb.mu.Unlock()
	return mb.SaveConfig()
}

func (mb *MCPBuilder) GetServer(id string) (*MCPServer, bool) {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	s, ok := mb.servers[id]
	return s, ok
}

func (mb *MCPBuilder) ListServers() []*MCPServer {
	mb.mu.RLock()
	defer mb.mu.RUnlock()
	list := make([]*MCPServer, 0, len(mb.servers))
	for _, s := range mb.servers {
		list = append(list, s)
	}
	return list
}

func (mb *MCPBuilder) StartServer(id string) error {
	mb.mu.Lock()
	server, ok := mb.servers[id]
	mb.mu.Unlock()
	if !ok {
		return fmt.Errorf("server %s not found", id)
	}

	if server.Status == "running" {
		return fmt.Errorf("server %s already running", id)
	}

	// Build the server
	cmd := exec.CommandContext(mb.ctx, "go", "build", "-o", "server", ".")
	cmd.Dir = server.WorkingDir
	if output, err := cmd.CombinedOutput(); err != nil {
		mb.mu.Lock()
		server.Status = "error"
		server.Metadata["last_error"] = string(output)
		mb.mu.Unlock()
		mb.SaveConfig()
		return fmt.Errorf("build failed: %s", output)
	}

	// Start the server
	runCmd := exec.CommandContext(mb.ctx, "./server")
	runCmd.Dir = server.WorkingDir
	runCmd.Env = append(os.Environ(), "MCP_TRANSPORT="+server.Transport)
	for k, v := range server.Env {
		runCmd.Env = append(runCmd.Env, k+"="+v)
	}

	if err := runCmd.Start(); err != nil {
		mb.mu.Lock()
		server.Status = "error"
		mb.mu.Unlock()
		mb.SaveConfig()
		return err
	}

	mb.mu.Lock()
	mb.processes[id] = runCmd
	server.Status = "running"
	server.PID = runCmd.Process.Pid
	server.StartedAt = time.Now()
	mb.mu.Unlock()

	mb.wg.Add(1)
	go func() {
		defer mb.wg.Done()
		runCmd.Wait()
		mb.mu.Lock()
		if s, ok := mb.servers[id]; ok {
			s.Status = "stopped"
			s.PID = 0
		}
		delete(mb.processes, id)
		mb.mu.Unlock()
		mb.SaveConfig()
	}()

	mb.SaveConfig()
	return nil
}

func (mb *MCPBuilder) StopServer(id string) error {
	mb.mu.Lock()
	cmd, ok := mb.processes[id]
	mb.mu.Unlock()
	if !ok {
		return fmt.Errorf("server %s not running", id)
	}

	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		cmd.Process.Kill()
	}

	mb.mu.Lock()
	if s, ok := mb.servers[id]; ok {
		s.Status = "stopping"
	}
	mb.mu.Unlock()
	return nil
}

func (mb *MCPBuilder) RestartServer(id string) error {
	if err := mb.StopServer(id); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return mb.StartServer(id)
}

func (mb *MCPBuilder) BuildServer(id string) error {
	mb.mu.RLock()
	server, ok := mb.servers[id]
	mb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server %s not found", id)
	}

	cmd := exec.CommandContext(mb.ctx, "go", "build", "-o", "server", ".")
	cmd.Dir = server.WorkingDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("build failed: %s", output)
	}
	return nil
}

func (mb *MCPBuilder) InstallDependencies(id string) error {
	mb.mu.RLock()
	server, ok := mb.servers[id]
	mb.mu.RUnlock()
	if !ok {
		return fmt.Errorf("server %s not found", id)
	}

	cmd := exec.CommandContext(mb.ctx, "go", "mod", "tidy")
	cmd.Dir = server.WorkingDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy failed: %s", output)
	}
	return nil
}

func (mb *MCPBuilder) GetServerLogs(id string, lines int) (string, error) {
	// In production, would capture stdout/stderr
	return fmt.Sprintf("Logs for server %s (last %d lines)", id, lines), nil
}

func main() {
	var (
		configPath = flag.String("config", "./config.yaml", "Path to config file")
		action     = flag.String("action", "", "Action: create, remove, list, start, stop, restart, build, install, logs, template")
		name       = flag.String("name", "", "Server name (for create)")
		description = flag.String("description", "", "Server description (for create)")
		transport  = flag.String("transport", "stdio", "Transport: stdio, sse, websocket")
		id         = flag.String("id", "", "Server ID")
		toolName   = flag.String("tool", "", "Tool name (for create)")
		toolDesc   = flag.String("tool-desc", "", "Tool description")
	)
	flag.Parse()

	mb, err := NewMCPBuilder(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize MCPBuilder: %v", err)
	}
	defer mb.cancel()

	switch *action {
	case "create":
		if *name == "" {
			log.Fatal("name required for create")
		}
		var tools []Tool
		if *toolName != "" {
			tools = append(tools, Tool{
				Name:        *toolName,
				Description: *toolDesc,
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"input": map[string]interface{}{"type": "string"},
					},
					"required": []string{"input"},
				},
			})
		}
		server := mb.CreateServer(*name, *description, *transport, tools)
		fmt.Printf("Created server %s (ID: %s)\n", server.Name, server.ID)
		fmt.Printf("Files generated in: %s\n", server.WorkingDir)

	case "remove":
		if *id == "" {
			log.Fatal("id required for remove")
		}
		if err := mb.RemoveServer(*id); err != nil {
			log.Fatalf("Failed to remove server: %v", err)
		}
		fmt.Printf("Removed server %s\n", *id)

	case "list":
		servers := mb.ListServers()
		if len(servers) == 0 {
			fmt.Println("No servers configured")
			return
		}
		for _, s := range servers {
			fmt.Printf("%s | %s | %s | %s | tools=%d\n", s.ID[:8], s.Name, s.Transport, s.Status, len(s.Tools))
		}

	case "start":
		if *id == "" {
			log.Fatal("id required for start")
		}
		if err := mb.StartServer(*id); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
		fmt.Printf("Started server %s\n", *id)

	case "stop":
		if *id == "" {
			log.Fatal("id required for stop")
		}
		if err := mb.StopServer(*id); err != nil {
			log.Fatalf("Failed to stop server: %v", err)
		}
		fmt.Printf("Stopped server %s\n", *id)

	case "restart":
		if *id == "" {
			log.Fatal("id required for restart")
		}
		if err := mb.RestartServer(*id); err != nil {
			log.Fatalf("Failed to restart server: %v", err)
		}
		fmt.Printf("Restarted server %s\n", *id)

	case "build":
		if *id == "" {
			log.Fatal("id required for build")
		}
		if err := mb.BuildServer(*id); err != nil {
			log.Fatalf("Failed to build server: %v", err)
		}
		fmt.Printf("Built server %s\n", *id)

	case "install":
		if *id == "" {
			log.Fatal("id required for install")
		}
		if err := mb.InstallDependencies(*id); err != nil {
			log.Fatalf("Failed to install dependencies: %v", err)
		}
		fmt.Printf("Installed dependencies for server %s\n", *id)

	case "logs":
		if *id == "" {
			log.Fatal("id required for logs")
		}
		logs, _ := mb.GetServerLogs(*id, 50)
		fmt.Println(logs)

	case "template":
		// Create a new template
		fmt.Println("Template creation not yet implemented")

	default:
		flag.Usage()
		os.Exit(1)
	}
}