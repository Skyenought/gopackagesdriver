package utils

import (
	"errors"
	"go/ast"
	"go/types"
	"sync"
	_ "unsafe"

	"golang.org/x/tools/go/packages"
)

//go:linkname loaderPackage golang.org/x/tools/go/packages.loaderPackage
type loaderPackage struct{}

//go:linkname parseValue golang.org/x/tools/go/packages.parseValue
type parseValue struct {
	f     *ast.File
	err   error
	ready chan struct{}
}

//go:linkname loader golang.org/x/tools/go/packages.loader
type loader struct {
	pkgs map[string]*loaderPackage
	packages.Config
	sizes        types.Sizes // non-nil if needed by mode
	parseCache   map[string]*parseValue
	parseCacheMu sync.Mutex
	exportMu     sync.Mutex // enforces mutual exclusion of exportdata operations

	// Config.Mode contains the implied mode (see impliedLoadMode).
	// Implied mode contains all the fields we need the data for.
	// In requestedMode there are the actually requested fields.
	// We'll zero them out before returning packages to the user.
	// This makes it easier for us to get the conditions where
	// we need certain modes right.
	requestedMode packages.LoadMode
}

//go:linkname driver golang.org/x/tools/go/packages.driver
type driver func(cfg *packages.Config, patterns ...string) (*packages.DriverResponse, error)

func UnsafeGetDriverResponse(cfg *packages.Config, patterns ...string) (*packages.DriverResponse, error) {
	ld := newLoader(cfg)
	const (
		// windowsArgMax specifies the maximum command line length for
		// the Windows' CreateProcess function.
		windowsArgMax = 32767
		// maxEnvSize is a very rough estimation of the maximum environment
		// size of a user.
		maxEnvSize = 16384
		// safeArgMax specifies the maximum safe command line length to use
		// by the underlying driver excl. the environment. We choose the Windows'
		// ARG_MAX as the starting point because it's one of the lowest ARG_MAX
		// constants out of the different supported platforms,
		// e.g., https://www.in-ulm.de/~mascheck/various/argmax/#results.
		safeArgMax = windowsArgMax - maxEnvSize
	)
	chunks, err := splitIntoChunks(patterns, safeArgMax)
	if err != nil {
		return nil, err
	}

	response, err := callDriverOnChunks(goListDriver, &ld.Config, chunks)
	if err != nil {
		return nil, err
	}
	return response, err
}

//go:linkname newLoader golang.org/x/tools/go/packages.newLoader
func newLoader(cfg *packages.Config) *loader

//go:linkname callDriverOnChunks golang.org/x/tools/go/packages.callDriverOnChunks
func callDriverOnChunks(driver driver, cfg *packages.Config, chunks [][]string) (*packages.DriverResponse, error)

//go:linkname goListDriver golang.org/x/tools/go/packages.goListDriver
func goListDriver(cfg *packages.Config, patterns ...string) (_ *packages.DriverResponse, err error)

func splitIntoChunks(patterns []string, argMax int) ([][]string, error) {
	if argMax <= 0 {
		return nil, errors.New("failed to split patterns into chunks, negative safe argMax value")
	}
	var chunks [][]string
	charsInChunk := 0
	nextChunkStart := 0
	for i, v := range patterns {
		vChars := len(v)
		if vChars > argMax {
			// a single pattern is longer than the maximum safe ARG_MAX, hardly should happen
			return nil, errors.New("failed to split patterns into chunks, a pattern is too long")
		}
		charsInChunk += vChars + 1 // +1 is for a whitespace between patterns that has to be counted too
		if charsInChunk > argMax {
			chunks = append(chunks, patterns[nextChunkStart:i])
			nextChunkStart = i
			charsInChunk = vChars
		}
	}
	// add the last chunk
	if nextChunkStart < len(patterns) {
		chunks = append(chunks, patterns[nextChunkStart:])
	}
	return chunks, nil
}
