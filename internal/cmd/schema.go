package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/invopop/jsonschema"
	"github.com/marang/franz-agent/internal/config"
	"github.com/spf13/cobra"
)

type configSchema struct {
	Schema string `json:"$schema,omitempty"`

	Models map[config.SelectedModelType]config.SelectedModel `json:"models,omitempty" jsonschema:"description=Model configurations for different model types,example={\"large\":{\"model\":\"gpt-4o\",\"provider\":\"openai\"}}"`

	Providers map[string]config.ProviderConfig `json:"providers,omitempty" jsonschema:"description=AI provider configurations"`

	MCP config.MCPs `json:"mcp,omitempty" jsonschema:"description=Model Context Protocol server configurations"`

	LSP config.LSPs `json:"lsp,omitempty" jsonschema:"description=Language Server Protocol configurations"`

	Options *config.Options `json:"options,omitempty" jsonschema:"description=General application options"`

	Permissions *config.Permissions `json:"permissions,omitempty" jsonschema:"description=Permission settings for tool usage"`

	Tools config.Tools `json:"tools,omitzero" jsonschema:"description=Tool configurations"`
}

const (
	schemaID      = "https://raw.githubusercontent.com/marang/franz-agent/main/schema.json"
	schemaRootRef = "#/$defs/Config"
)

var schemaCmd = &cobra.Command{
	Use:    "schema",
	Short:  "Generate JSON schema for configuration",
	Long:   "Generate JSON schema for the franz-agent configuration file",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		reflector := new(jsonschema.Reflector)
		schema := reflector.Reflect(&configSchema{})
		schema.ID = jsonschema.ID(schemaID)
		if root, ok := schema.Definitions["configSchema"]; ok {
			schema.Ref = schemaRootRef
			schema.Definitions["Config"] = root
			delete(schema.Definitions, "configSchema")
		}

		bts, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal schema: %w", err)
		}
		fmt.Println(string(bts))
		return nil
	},
}
