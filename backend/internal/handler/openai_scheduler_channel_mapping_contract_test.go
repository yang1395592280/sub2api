package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIEntryPointsResolveChannelMappedModelBeforeScheduling(t *testing.T) {
	tests := []struct {
		file              string
		legacySelector    string
		resolverSelector  string
		wantResolverCalls int
	}{
		{
			file:              "openai_gateway_handler.go",
			legacySelector:    "SelectEffectiveOpenAIAccountWithSchedulerForCapability",
			resolverSelector:  "SelectEffectiveOpenAIAccountWithSchedulerForCapabilityAndModelResolver",
			wantResolverCalls: 3,
		},
		{
			file:              "openai_chat_completions.go",
			legacySelector:    "SelectEffectiveOpenAIAccountWithSchedulerForCapability",
			resolverSelector:  "SelectEffectiveOpenAIAccountWithSchedulerForCapabilityAndModelResolver",
			wantResolverCalls: 1,
		},
		{
			file:              "openai_embeddings.go",
			legacySelector:    "SelectEffectiveOpenAIAccountWithSchedulerForCapability",
			resolverSelector:  "SelectEffectiveOpenAIAccountWithSchedulerForCapabilityAndModelResolver",
			wantResolverCalls: 1,
		},
		{
			file:              "openai_images.go",
			legacySelector:    "SelectEffectiveOpenAIAccountWithSchedulerForImages",
			resolverSelector:  "SelectEffectiveOpenAIAccountWithSchedulerForImagesAndModelResolver",
			wantResolverCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join(".", tt.file), nil, 0)
			require.NoError(t, err)

			legacyCalls := 0
			resolverCalls := 0
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch selector.Sel.Name {
				case tt.legacySelector:
					legacyCalls++
				case tt.resolverSelector:
					resolverCalls++
				}
				return true
			})

			require.Zero(t, legacyCalls, "entry point must not schedule with the unmapped client model")
			require.Equal(t, tt.wantResolverCalls, resolverCalls)
		})
	}
}
