package jtbd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var servesRe = regexp.MustCompile(`Serves:\s*(.+)`)

// Scan walks root for _test.go files and extracts // Serves: annotations
// from doc comments on Test* functions. modulePath is the Go module path.
func Scan(root, modulePath string) ([]Annotation, error) {
	var annotations []Annotation

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	err = filepath.Walk(absRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// Don't skip the root directory itself, only subdirectories.
			if path != absRoot {
				switch info.Name() {
				case ".git", "vendor", "testdata", ".worktrees", "tools":
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fa, scanErr := scanFile(path, absRoot, modulePath)
		if scanErr != nil {
			return nil // skip unparseable files
		}
		annotations = append(annotations, fa...)
		return nil
	})

	return annotations, err
}

func scanFile(path, root, modulePath string) ([]Annotation, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(root, filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	pkgPath := modulePath
	if rel != "." {
		pkgPath = modulePath + "/" + filepath.ToSlash(rel)
	}

	var annotations []Annotation

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc == nil {
			continue
		}
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		for _, comment := range fn.Doc.List {
			matches := servesRe.FindStringSubmatch(comment.Text)
			if len(matches) < 2 {
				continue
			}
			ids := parseIDs(matches[1])
			if len(ids) > 0 {
				annotations = append(annotations, Annotation{
					Package:  pkgPath,
					FuncName: fn.Name.Name,
					JobIDs:   ids,
				})
			}
		}
	}

	return annotations, nil
}

func parseIDs(s string) []string {
	parts := strings.Split(s, ",")
	var ids []string
	for _, p := range parts {
		id := strings.TrimSpace(p)
		if id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
