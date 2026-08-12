package main

import (
	"fmt"
	"strings"

	"github.com/lin-br/go-linai-tools/internal/core/domain"
)

// renderTree renders a CodePlan as a human-readable ASCII tree. The tree shows
// the plan summary and language as a header, each file path as a top-level
// entry, and Types:/Functions: sections indented beneath each file. Type
// fields are listed as "Name: Type" lines. No emojis are used — ASCII
// indentation (spaces) conveys hierarchy. Empty Types or Functions slices
// omit the section label entirely.
func renderTree(plan *domain.CodePlan) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Code Plan: %s\n", plan.Summary)
	fmt.Fprintf(&sb, "Language: %s\n", plan.Language)

	for _, file := range plan.Files {
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%s\n", file.Path)

		if len(file.Types) > 0 {
			sb.WriteString("  Types:\n")
			for _, typ := range file.Types {
				fmt.Fprintf(&sb, "    %s\n", typ.Name)
				for _, field := range typ.Fields {
					fmt.Fprintf(&sb, "      %s: %s\n", field.Name, field.Type)
				}
			}
		}

		if len(file.Functions) > 0 {
			sb.WriteString("  Functions:\n")
			for _, fn := range file.Functions {
				fmt.Fprintf(&sb, "    %s\n", fn.Signature)
			}
		}
	}

	return sb.String()
}
