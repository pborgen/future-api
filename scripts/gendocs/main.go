// Command gendocs invokes swaggo/swag programmatically to generate the
// OpenAPI spec for the API. We do this in-process because the prebuilt swag
// CLI ships a stripped binary that crashes on recent macOS dyld versions.
//
// Usage: go run ./scripts/gendocs
package main

import (
	"log"

	"github.com/swaggo/swag/gen"
)

func main() {
	cfg := &gen.Config{
		SearchDir:          ".",
		Excludes:           "./scripts,./docs",
		MainAPIFile:        "cmd/server/main.go",
		PropNamingStrategy: "camelcase",
		OutputDir:          "./docs",
		OutputTypes:        []string{"go", "json", "yaml"},
		ParseDependency:    1,
		ParseInternal:      true,
		ParseDepth:         100,
		LeftTemplateDelim:  "{{",
		RightTemplateDelim: "}}",
	}
	if err := gen.New().Build(cfg); err != nil {
		log.Fatalf("swag gen: %v", err)
	}
	log.Println("docs generated in ./docs")
}
