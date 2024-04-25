package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/gopackagesdriver/internal/utils"
	"golang.org/x/tools/go/packages"
)

var (
	keyword                   = getenvDefault("CWGO_GOPACKAGESDRIVER_KEYWORD", "kitex_gen")
	allForOnePackagesJSONPath = getenvDefault("CWGO_GOPACKAGESDRIVER_MERGE", "/Users/skyenought/gopath/src/github.com/cloudwego/usage/cwgo_gopackagesdriver.json")
)

func main() {
	ctx, cancel := signalContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := run(ctx, os.Stdout, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		// gopls will check the packages driver exit code, and if there is an
		// error, it will fall back to go list. Obviously we don't want that,
		// so force a 0 exit code.
		os.Exit(0)
	}
}

func run(ctx context.Context, out io.Writer, args []string) error {
	var (
		_packages  []*packages.Package
		targetResp packages.DriverResponse
		err        error
	)

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedTypesSizes |
			packages.NeedModule |
			packages.NeedEmbedFiles,
		Context: ctx,
	}

	ret, err := utils.UnsafeGetDriverResponse(cfg, args...)
	if err != nil {
		return fmt.Errorf("get driver response error: %v", err)
	}

	var wg sync.WaitGroup

	for _, pkg := range ret.Packages {
		wg.Add(1)
		go func(pkg *packages.Package) {
			defer wg.Done()
			if !strings.HasPrefix(pkg.PkgPath, keyword) {
				_packages = append(_packages, pkg)
			}
		}(pkg)
	}

	wg.Wait()

	// TODO: cut point
	targetResp, err = getTargetPackages()
	if err != nil {
		return err
	}

	_packages = append(_packages, targetResp.Packages...)

	resp := &packages.DriverResponse{
		NotHandled: false,
		Compiler:   "gc",
		Arch:       runtime.GOARCH,
		Roots:      ret.Roots,
		Packages:   _packages,
	}

	data, err := sonic.ConfigFastest.Marshal(resp)
	if err != nil {
		return fmt.Errorf("json marshal error: %v", err.Error())
	}
	_, err = out.Write(data)
	return err
}

var (
	lastFileModTime time.Time
	lastFileHash    string
	lastTargetResp  packages.DriverResponse
)

// getTargetPackages 获取目标 []packages.Package
// 方法:
// - 确定目标文件夹位置
// - 使用 packages.GetDriverResponse 获取目标信息
// - 通过约定关键字确定需要的信息
// - 返回目标值
func getTargetPackages() (packages.DriverResponse, error) {
	// TODO: env var
	file := allForOnePackagesJSONPath
	// 获取文件信息
	fileInfo, err := os.Stat(file)
	if err != nil {
		return packages.DriverResponse{}, err
	}

	// 检查文件修改时间是否变化
	if lastFileModTime.Equal(fileInfo.ModTime()) {
		return lastTargetResp, nil
	}

	// 计算文件哈希值
	fileHash, err := calculateFileHash(file)
	if err != nil {
		return packages.DriverResponse{}, err
	}

	// 检查文件哈希值是否变化
	if lastFileHash == fileHash {
		return lastTargetResp, err
	}

	// 文件内容发生变化，读取文件并更新记录
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return packages.DriverResponse{}, err
	}

	// 解析文件内容
	var targetResp packages.DriverResponse
	if err := sonic.ConfigFastest.Unmarshal(fileContent, &targetResp); err != nil {
		return packages.DriverResponse{}, err
	}

	// 更新记录
	lastFileModTime = fileInfo.ModTime()
	lastFileHash = fileHash
	lastTargetResp = targetResp

	return targetResp, err
}

func calculateFileHash(file string) (string, error) {
	// 打开文件
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// 计算文件哈希值
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	// 返回哈希值的字符串表示
	return hex.EncodeToString(h.Sum(nil)), nil
}

func signalContext(parentCtx context.Context, signals ...os.Signal) (ctx context.Context, stop context.CancelFunc) {
	ctx, cancel := context.WithCancel(parentCtx)
	ch := make(chan os.Signal, 1)
	go func() {
		select {
		case <-ch:
			cancel()
		case <-ctx.Done():
		}
	}()
	signal.Notify(ch, signals...)

	return ctx, cancel
}

func getenvDefault(key, defaultValue string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return defaultValue
}
