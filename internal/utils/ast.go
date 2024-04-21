package utils

import (
	"go/parser"
	"go/token"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

func GetFileImports(path string) (map[string]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, strings.Split(path, "/")[len(strings.Split(path, "/"))-1], path, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	importMap := make(map[string]string)
	imports := astutil.Imports(fset, node)
	for _, importSpecs := range imports {
		for _, is := range importSpecs {
			importStr := strings.Trim(is.Path.Value, "\"")
			importMap[importStr] = importStr
		}
	}
	return importMap, nil
}

func GetGoMod(path string) (string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "", path, parser.ParseComments)
	if err != nil {
		return "", err
	}
	for _, commentGroup := range node.Comments {
		for _, comment := range commentGroup.List {
			splits := strings.Split(comment.Text, "=")
			if len(splits) == 2 && strings.Contains(splits[0], "mod") {
				return splits[1], nil
			}
		}
	}
	return "", nil
}
