// Package verify implements deterministic contract and schema parity verification.
package verify

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/mgantlett/nomos/substrate/src/nomos/core/config"
)

func verifyNomosSchemaArtifact(root string, data []byte) string {
	selfHostedConfigGo := filepath.Join(root, "substrate", "src", "nomos", "core", "config", "config.go")
	if _, err := os.Stat(selfHostedConfigGo); err != nil {
		if pErr := config.VerifyConfigJSONSchemaParity(data); pErr != nil {
			return pErr.Error()
		}
		return ""
	}
	return verifySelfHostedNomosSchemaAST(selfHostedConfigGo, data)
}

func verifySelfHostedNomosSchemaAST(configGoPath string, schemaBytes []byte) string {
	goFields, err := extractGoStructJSONTags(configGoPath)
	if err != nil {
		return fmt.Sprintf("failed parsing %s AST: %v", configGoPath, err)
	}
	var rootNode config.JSONSchemaNode
	if uErr := json.Unmarshal(schemaBytes, &rootNode); uErr != nil {
		return fmt.Sprintf("invalid nomos.schema.json structure: %v", uErr)
	}
	if rootNode.Schema != config.JSONSchemaDraft202012URI {
		return "nomos.schema.json missing canonical Draft 2020-12 $schema URI"
	}
	schemaProps := make(map[string]bool)
	collectSchemaPropertyKeys(&rootNode, schemaProps)

	var missingInSchema []string
	for k := range goFields {
		if !schemaProps[k] {
			missingInSchema = append(missingInSchema, k)
		}
	}
	var extraInSchema []string
	for k := range schemaProps {
		if !goFields[k] {
			extraInSchema = append(extraInSchema, k)
		}
	}
	sort.Strings(missingInSchema)
	sort.Strings(extraInSchema)
	if len(missingInSchema) > 0 || len(extraInSchema) > 0 {
		return fmt.Sprintf("nomos.schema.json has drifted from worktree config.EcosystemConfig AST (missing=%v, extra=%v)", missingInSchema, extraInSchema)
	}
	return ""
}

func extractGoStructJSONTags(configGoPath string) (map[string]bool, error) {
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, configGoPath, nil, 0)
	if err != nil {
		return nil, err
	}
	tags := make(map[string]bool)
	ast.Inspect(fileAST, func(n ast.Node) bool {
		st, ok := n.(*ast.StructType)
		if !ok || st.Fields == nil {
			return true
		}
		for _, field := range st.Fields.List {
			if name := parseFieldJSONTagName(field); name != "" {
				tags[name] = true
			}
		}
		return true
	})
	return tags, nil
}

func parseFieldJSONTagName(field *ast.Field) string {
	if field == nil || field.Tag == nil {
		return ""
	}
	rawTag := strings.Trim(field.Tag.Value, "`")
	jsonTag := reflect.StructTag(rawTag).Get("json")
	if jsonTag == "" || jsonTag == "-" {
		return ""
	}
	name := strings.TrimSpace(strings.Split(jsonTag, ",")[0])
	if name == "-" {
		return ""
	}
	return name
}

func collectSchemaPropertyKeys(node *config.JSONSchemaNode, out map[string]bool) {
	if node == nil {
		return
	}
	for k, child := range node.Properties {
		out[k] = true
		collectSchemaPropertyKeys(child, out)
	}
	if node.Items != nil {
		collectSchemaPropertyKeys(node.Items, out)
	}
}
