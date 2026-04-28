package installer

import (
	_ "embed"
	"path/filepath"

	"github.com/ranwei/mneme/pkg/state"
)

//go:embed templates/reframe.md.tmpl
var reframeTemplate string

// InstallReframe writes the bundled Reframe knowledge base to
// <projectRoot>/.mneme/reframe.md, overwriting any existing copy.
func InstallReframe(projectRoot string) error {
	dst := filepath.Join(projectRoot, ".mneme", "reframe.md")
	return state.AtomicWrite(dst, []byte(reframeTemplate))
}
