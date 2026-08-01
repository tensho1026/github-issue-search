// Command check-godoc enforces the repository's Go documentation contract.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type packageDocumentation struct {
	directory string
	name      string
	document  string
}

func main() {
	root := "apps/api"
	if len(os.Args) == 2 {
		root = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run scripts/ci/check-godoc.go [go-source-root]")
		os.Exit(2)
	}

	issues, declarationCount, packageCount, err := inspect(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "inspect GoDoc: %v\n", err)
		os.Exit(1)
	}
	if len(issues) > 0 {
		fmt.Fprintln(os.Stderr, "GoDoc policy violations:")
		for _, issue := range issues {
			fmt.Fprintf(os.Stderr, "- %s\n", issue)
		}
		os.Exit(1)
	}

	fmt.Printf(
		"%d exported declarations across %d packages passed GoDoc policy checks.\n",
		declarationCount,
		packageCount,
	)
}

func inspect(root string) ([]string, int, int, error) {
	fileSet := token.NewFileSet()
	packages := make(map[string]packageDocumentation)
	issues := make([]string, 0)
	declarationCount := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fileSet, path, nil, parser.ParseComments)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		if ast.IsGenerated(file) {
			return nil
		}

		directory := filepath.Dir(path)
		current := packages[directory]
		current.directory = directory
		current.name = file.Name.Name
		if file.Doc != nil && strings.TrimSpace(file.Doc.Text()) != "" {
			current.document = strings.TrimSpace(file.Doc.Text())
		}
		packages[directory] = current

		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if !value.Name.IsExported() {
					continue
				}
				if value.Recv != nil &&
					!receiverIsExported(value.Recv.List[0].Type) {
					continue
				}
				declarationCount++
				issues = append(
					issues,
					validateComment(
						fileSet.Position(value.Pos()),
						value.Name.Name,
						value.Doc,
					)...,
				)
			case *ast.GenDecl:
				if value.Tok != token.CONST &&
					value.Tok != token.TYPE &&
					value.Tok != token.VAR {
					continue
				}
				for _, specification := range value.Specs {
					switch item := specification.(type) {
					case *ast.TypeSpec:
						if !item.Name.IsExported() {
							continue
						}
						declarationCount++
						issues = append(
							issues,
							validateComment(
								fileSet.Position(item.Pos()),
								item.Name.Name,
								firstComment(item.Doc, value.Doc, item.Comment),
							)...,
						)
						interfaceType, isInterface := item.Type.(*ast.InterfaceType)
						if !isInterface {
							continue
						}
						for _, method := range interfaceType.Methods.List {
							for _, name := range method.Names {
								if !name.IsExported() {
									continue
								}
								declarationCount++
								issues = append(
									issues,
									validateComment(
										fileSet.Position(name.Pos()),
										name.Name,
										firstComment(method.Doc, method.Comment),
									)...,
								)
							}
						}
					case *ast.ValueSpec:
						for _, name := range item.Names {
							if !name.IsExported() {
								continue
							}
							declarationCount++
							comment := firstComment(item.Doc, value.Doc, item.Comment)
							if len(value.Specs) > 1 && value.Doc != nil {
								if strings.TrimSpace(value.Doc.Text()) != "" {
									continue
								}
							}
							issues = append(
								issues,
								validateComment(
									fileSet.Position(name.Pos()),
									name.Name,
									comment,
								)...,
							)
						}
					}
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, 0, 0, err
	}

	for _, documentation := range packages {
		position := documentation.directory
		switch {
		case documentation.document == "":
			issues = append(issues, position+": package has no package comment")
		case documentation.name == "main":
			commandName := filepath.Base(documentation.directory)
			if !strings.HasPrefix(documentation.document, "Command "+commandName) {
				issues = append(
					issues,
					position+": command comment must start with \"Command "+commandName+"\"",
				)
			}
		case !strings.HasPrefix(
			documentation.document,
			"Package "+documentation.name,
		):
			issues = append(
				issues,
				position+": package comment must start with \"Package "+
					documentation.name+"\"",
			)
		}
	}

	slices.Sort(issues)
	return issues, declarationCount, len(packages), nil
}

func firstComment(groups ...*ast.CommentGroup) *ast.CommentGroup {
	for _, group := range groups {
		if group != nil && strings.TrimSpace(group.Text()) != "" {
			return group
		}
	}
	return nil
}

func validateComment(
	position token.Position,
	name string,
	group *ast.CommentGroup,
) []string {
	location := fmt.Sprintf("%s:%d", position.Filename, position.Line)
	if group == nil {
		return []string{location + ": " + name + " has no GoDoc comment"}
	}
	text := strings.TrimSpace(group.Text())
	if text == "" {
		return []string{location + ": " + name + " has an empty GoDoc comment"}
	}
	if !strings.HasPrefix(text, name) {
		return []string{
			location + ": " + name + " comment must start with the declaration name",
		}
	}
	return nil
}

func receiverIsExported(expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.Ident:
		return value.IsExported()
	case *ast.IndexExpr:
		return receiverIsExported(value.X)
	case *ast.IndexListExpr:
		return receiverIsExported(value.X)
	case *ast.ParenExpr:
		return receiverIsExported(value.X)
	case *ast.StarExpr:
		return receiverIsExported(value.X)
	default:
		return false
	}
}
