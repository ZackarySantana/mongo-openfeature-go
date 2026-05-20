package editor

import (
	"os"
	"path/filepath"
	"runtime"
)

// mustChdirToRoot hops up from this package directory to the module root so
// template paths like "internal/editor/layout.tmpl" resolve during tests.
// Called from package-level init in the test files; safe to call multiple
// times. chdir is idempotent for a given target.
func mustChdirToRoot() {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("could not determine current test file path")
	}
	// internal/editor/<file> -> module root is two parents up.
	root := filepath.Join(filepath.Dir(thisFile), "..", "..")
	if err := os.Chdir(root); err != nil {
		panic("could not chdir to module root: " + err.Error())
	}
}
