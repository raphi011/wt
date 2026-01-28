package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// TestFunc represents a parsed test function.
type TestFunc struct {
	Name    string // Function name (e.g., "TestCheckout_ExistingBranch")
	Doc     string // Doc comment text
	Line    int    // Line number in source file
	IsTable bool   // Whether this appears to be a table-driven test
}

// TestFile represents a parsed test file.
type TestFile struct {
	Name  string     // File name (e.g., "checkout_integration_test.go")
	Path  string     // Full path to file
	Tests []TestFunc // Test functions in this file
}

// TestPackage represents a collection of test files in a package.
type TestPackage struct {
	Name       string     // Package import path relative to root
	Files      []TestFile // Test files in this package
	TotalTests int        // Total test count
}

// ParseTestFiles walks the directory tree and parses all *_test.go files.
// If integrationOnly is true, only files matching *_integration_test.go are included.
func ParseTestFiles(root string, integrationOnly bool) ([]TestPackage, error) {
	packageMap := make(map[string]*TestPackage)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and hidden directories
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process test files
		if !strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		// Filter to integration tests only if requested
		if integrationOnly && !strings.HasSuffix(info.Name(), "_integration_test.go") {
			return nil
		}

		// Parse the file
		testFile, err := parseTestFile(path)
		if err != nil {
			return err
		}

		// Skip files with no tests
		if len(testFile.Tests) == 0 {
			return nil
		}

		// Get package path relative to root
		dir := filepath.Dir(path)
		pkgPath, err := filepath.Rel(root, dir)
		if err != nil {
			pkgPath = dir
		}
		if pkgPath == "." {
			pkgPath = filepath.Base(root)
		}

		// Add to package map
		pkg, ok := packageMap[pkgPath]
		if !ok {
			pkg = &TestPackage{Name: pkgPath}
			packageMap[pkgPath] = pkg
		}
		pkg.Files = append(pkg.Files, *testFile)
		pkg.TotalTests += len(testFile.Tests)

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	packages := make([]TestPackage, 0, len(packageMap))
	for _, pkg := range packageMap {
		// Sort files within package
		sort.Slice(pkg.Files, func(i, j int) bool {
			return pkg.Files[i].Name < pkg.Files[j].Name
		})
		packages = append(packages, *pkg)
	}

	// Sort packages by name
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	return packages, nil
}

// parseTestFile parses a single test file and extracts test functions.
func parseTestFile(path string) (*TestFile, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	testFile := &TestFile{
		Name: filepath.Base(path),
		Path: path,
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Only include Test* functions
		if !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}

		// Skip helper functions (must have *testing.T or *testing.B parameter)
		if !isTestFunction(fn) {
			continue
		}

		testFunc := TestFunc{
			Name: fn.Name.Name,
			Line: fset.Position(fn.Pos()).Line,
		}

		// Extract doc comment
		if fn.Doc != nil {
			testFunc.Doc = strings.TrimSpace(fn.Doc.Text())
		}

		// Detect table-driven tests by looking for range loops with t.Run
		testFunc.IsTable = detectTableDriven(fn)

		testFile.Tests = append(testFile.Tests, testFunc)
	}

	return testFile, nil
}

// isTestFunction checks if the function signature matches a test function.
func isTestFunction(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || len(fn.Type.Params.List) != 1 {
		return false
	}

	param := fn.Type.Params.List[0]
	starExpr, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return false
	}

	selExpr, ok := starExpr.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selExpr.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "testing" && (selExpr.Sel.Name == "T" || selExpr.Sel.Name == "B")
}

// detectTableDriven attempts to detect if a test is table-driven.
func detectTableDriven(fn *ast.FuncDecl) bool {
	if fn.Body == nil {
		return false
	}

	isTable := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Look for range statements
		rangeStmt, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}

		// Check if body contains t.Run
		ast.Inspect(rangeStmt.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			if sel.Sel.Name == "Run" {
				isTable = true
				return false
			}
			return true
		})

		return !isTable
	})

	return isTable
}
