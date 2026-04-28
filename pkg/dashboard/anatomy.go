package dashboard

import (
	"net/http"
	"path/filepath"
	"sort"

	"github.com/ranwei/mneme/pkg/state"
)

type AnatomyFile struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	EstTokens   int    `json:"est_tokens"`
	Language    string `json:"language"`
}

type AnatomyDir struct {
	Path  string        `json:"path"`
	Files []AnatomyFile `json:"files"`
}

func AnatomyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "GET only", http.StatusMethodNotAllowed)
			return
		}
		_, root := resolveProject(w, r)
		if root == "" {
			return
		}
		entries, err := state.ReadAnatomy(root)
		if err != nil {
			http.Error(w, `{"error":"read_anatomy"}`, http.StatusInternalServerError)
			return
		}
		groups := map[string][]AnatomyFile{}
		for path, e := range entries {
			dir := filepath.Dir(path)
			groups[dir] = append(groups[dir], AnatomyFile{
				Name: filepath.Base(path), Description: e.Description,
				EstTokens: e.EstTokens, Language: e.Language,
			})
		}
		dirs := make([]string, 0, len(groups))
		for d := range groups {
			dirs = append(dirs, d)
		}
		sort.Strings(dirs)
		out := make([]AnatomyDir, 0, len(dirs))
		for _, d := range dirs {
			files := groups[d]
			sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
			out = append(out, AnatomyDir{Path: d, Files: files})
		}
		genTime, _ := state.ReadAnatomyGeneratedTime(root)
		writeJSON(w, 200, map[string]any{
			"directories":  out,
			"generated_at": genTime.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}
