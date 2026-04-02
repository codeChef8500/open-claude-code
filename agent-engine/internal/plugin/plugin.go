package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"net/rpc"
	"os/exec"

	goplugin "github.com/hashicorp/go-plugin"
	"github.com/wall-ai/agent-engine/internal/engine"
	"github.com/wall-ai/agent-engine/internal/tool"
)

// PluginMeta is the descriptor returned by a plugin during handshake.
type PluginMeta struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Version     string          `json:"version"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// HandshakeConfig is the hashicorp/go-plugin handshake configuration.
var HandshakeConfig = goplugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "AGENT_ENGINE_PLUGIN",
	MagicCookieValue: "wall-ai-agent-engine-v1",
}

// ExternalPlugin wraps a subprocess plugin binary as a Tool.
type ExternalPlugin struct {
	tool.BaseTool
	meta   PluginMeta
	client *goplugin.Client
	binary string
}

// LoadPlugin launches a plugin binary and retrieves its metadata.
func LoadPlugin(binaryPath string) (*ExternalPlugin, error) {
	client := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig: HandshakeConfig,
		Cmd:             exec.Command(binaryPath),
		Plugins: goplugin.PluginSet{
			"tool": &ToolPlugin{},
		},
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("plugin connect: %w", err)
	}

	raw, err := rpcClient.Dispense("tool")
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("plugin dispense: %w", err)
	}

	// Request metadata from the plugin.
	tp, ok := raw.(ToolPluginInterface)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("plugin does not implement ToolPluginInterface")
	}

	metaJSON, err := tp.Metadata()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("plugin metadata: %w", err)
	}

	var meta PluginMeta
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		client.Kill()
		return nil, fmt.Errorf("plugin metadata parse: %w", err)
	}

	return &ExternalPlugin{meta: meta, client: client, binary: binaryPath}, nil
}

// Close terminates the plugin subprocess.
func (p *ExternalPlugin) Close() { p.client.Kill() }

// ─── Tool interface implementation ────────────────────────────────────────────

func (p *ExternalPlugin) Name() string                      { return p.meta.Name }
func (p *ExternalPlugin) UserFacingName() string            { return p.meta.Name }
func (p *ExternalPlugin) Description() string               { return p.meta.Description }
func (p *ExternalPlugin) IsReadOnly() bool                  { return false }
func (p *ExternalPlugin) IsConcurrencySafe() bool           { return false }
func (p *ExternalPlugin) MaxResultSizeChars() int           { return 50_000 }
func (p *ExternalPlugin) IsEnabled(_ *tool.UseContext) bool { return true }
func (p *ExternalPlugin) InputSchema() json.RawMessage      { return p.meta.InputSchema }
func (p *ExternalPlugin) Prompt(_ *tool.UseContext) string  { return "" }

func (p *ExternalPlugin) CheckPermissions(_ context.Context, _ json.RawMessage, _ *tool.UseContext) error {
	return nil
}

func (p *ExternalPlugin) Call(ctx context.Context, input json.RawMessage, _ *tool.UseContext) (<-chan *engine.ContentBlock, error) {
	ch := make(chan *engine.ContentBlock, 4)
	go func() {
		defer close(ch)

		rpcClient, err := p.client.Client()
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		raw, err := rpcClient.Dispense("tool")
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		tp := raw.(ToolPluginInterface)
		result, err := tp.Call(input)
		if err != nil {
			ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: err.Error(), IsError: true}
			return
		}
		ch <- &engine.ContentBlock{Type: engine.ContentTypeText, Text: string(result)}
	}()
	return ch, nil
}

// ─── Plugin interfaces (implemented by plugin subprocess) ─────────────────────

// ToolPluginInterface is the RPC interface that plugin binaries must implement.
type ToolPluginInterface interface {
	Metadata() ([]byte, error)
	Call(input []byte) ([]byte, error)
}

// ToolPlugin implements hashicorp/go-plugin's Plugin interface.
type ToolPlugin struct {
	Impl ToolPluginInterface
}

func (p *ToolPlugin) Server(*goplugin.MuxBroker) (interface{}, error) {
	return &ToolPluginRPCServer{Impl: p.Impl}, nil
}

func (p *ToolPlugin) Client(_ *goplugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	return &ToolPluginRPCClient{client: c}, nil
}

// ─── RPC server/client stubs ─────────────────────────────────────────────────

type ToolPluginRPCServer struct{ Impl ToolPluginInterface }

func (s *ToolPluginRPCServer) Metadata(_ interface{}, reply *[]byte) error {
	b, err := s.Impl.Metadata()
	*reply = b
	return err
}

func (s *ToolPluginRPCServer) Call(args []byte, reply *[]byte) error {
	b, err := s.Impl.Call(args)
	*reply = b
	return err
}

type ToolPluginRPCClient struct{ client *rpc.Client }

func (c *ToolPluginRPCClient) Metadata() ([]byte, error) {
	var reply []byte
	err := c.client.Call("Plugin.Metadata", new(interface{}), &reply)
	return reply, err
}

func (c *ToolPluginRPCClient) Call(input []byte) ([]byte, error) {
	var reply []byte
	err := c.client.Call("Plugin.Call", input, &reply)
	return reply, err
}
