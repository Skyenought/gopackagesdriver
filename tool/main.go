package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/gopackagesdriver/internal/utils"
	"golang.org/x/tools/go/packages"
)

var (
	pattern                   string
	keyword                   string
	allForOnePackagesJSONPath string
)

func init() {
	flag.StringVar(&pattern, "pattern", "", "")
	flag.StringVar(&keyword, "keyword", "kitex_gen", "")
	flag.StringVar(&allForOnePackagesJSONPath, "merge", "", "")

	keyword = getenvDefault("CWGO_GOPACKAGESDRIVER_KEYWORD", "kitex_gen")
	allForOnePackagesJSONPath = getenvDefault("CWGO_GOPACKAGESDRIVER_MERGE", "/Users/skyenought/gopath/src/cwgo_all.json")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "[error]: %v\n", err.Error())
		os.Exit(0)
	}
}

type entity struct {
	Pkgs []*packages.Package `json:"packages"`
}

func run() error {
	flag.Parse()

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypesSizes |
			packages.NeedModule |
			packages.NeedEmbedFiles,
	}
	var (
		hasKeywordJSONDirs []string
		err                error
	)

	// pattern 下所有含有 go.mod 的文件夹
	dirs, err := findGoModDirs(pattern)
	if err != nil {
		return err
	}

	// 含有 keyword 的 pkgPath 记录下来
	for _, dir := range dirs {
		err = os.Chdir(dir)
		if err != nil {
			return fmt.Errorf("failed to change directory to %s: %v", dir, err)
		}
		fmt.Printf("Changed working directory to %s\n", dir)

		response, err := utils.UnsafeGetDriverResponse(cfg, "./...")
		if err != nil {
			return fmt.Errorf("get driver response fail, %v", err.Error())
		}

		var targetPackages []*packages.Package // Reset targetPackages for each iteration

		for _, pkg := range response.Packages {
			if strings.Contains(pkg.PkgPath, keyword) {
				targetPackages = append(targetPackages, pkg)
			}
		}

		if len(targetPackages) > 0 {
			hasKeywordJSONDirs = append(hasKeywordJSONDirs, dir)

			file, err := os.Create("./cwgo_gopackagesdriver.json")
			if err != nil {
				return fmt.Errorf("write json fail, %v", err)
			}

			bytes, _ := sonic.ConfigFastest.Marshal(map[string][]*packages.Package{
				"packages": targetPackages,
			})

			file.Write(bytes)
			file.Close()
		}
	}

	pkgMap := make(map[string]*packages.Package)

	for _, dir := range hasKeywordJSONDirs {
		jsonpath := filepath.Join(dir, "cwgo_gopackagesdriver.json")
		jsonFile, _ := os.ReadFile(jsonpath)

		var tmpEntity entity
		sonic.ConfigFastest.Unmarshal(jsonFile, &tmpEntity)
		for _, pkg := range tmpEntity.Pkgs {
			pkgMap[pkg.PkgPath] = pkg
		}
	}

	var _packages []*packages.Package

	for _, pkg := range pkgMap {
		_packages = append(_packages, pkg)
	}

	file, err := os.Create(allForOnePackagesJSONPath)
	if err != nil {
		return fmt.Errorf("write json fail, %v", err)
	}

	bytes, _ := sonic.ConfigFastest.Marshal(map[string][]*packages.Package{
		"packages": _packages,
	})
	file.Write(bytes)
	return nil
}

func getenvDefault(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}

func findGoModDirs(path string) ([]string, error) {
	var dirs []string
	err := filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "go.mod" {
			path := strings.TrimSuffix(path, "/go.mod")
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dirs, nil
}
