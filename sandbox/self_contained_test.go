package sandbox

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSDKSelfContained_NoBackendImports(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve runtime caller")
	}
	root := filepath.Dir(thisFile)

	files, err := filepath.Glob(filepath.Join(root, "*.go"))
	if err != nil {
		t.Fatalf("glob sdk files: %v", err)
	}

	fset := token.NewFileSet()
	for _, file := range files {
		parsed, parseErr := parser.ParseFile(fset, file, nil, parser.ImportsOnly)
		if parseErr != nil {
			t.Fatalf("parse %s imports: %v", filepath.Base(file), parseErr)
		}

		for _, imp := range parsed.Imports {
			path := strings.Trim(imp.Path.Value, "\"")
			if strings.Contains(path, "luxorlabs/tenki-cloud") || strings.Contains(path, "/backend/") {
				t.Fatalf("sdk must not import backend package %q in %s", path, filepath.Base(file))
			}
		}
	}
}
