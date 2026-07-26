package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateLiveCallUsesEffectiveOpenAIGroupScheduling(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(".", "openai_live.go"), nil, 0)
	require.NoError(t, err)

	directCalls := 0
	effectiveCalls := 0
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
		case "SelectAccountWithSchedulerForCapability":
			directCalls++
		case "SelectEffectiveOpenAIAccountWithSchedulerForCapability":
			effectiveCalls++
		}
		return true
	})

	require.Zero(t, directCalls, "Live must not bypass automatic-cheapest and self-hosted-pool source stages")
	require.Equal(t, 1, effectiveCalls)
}
