package plugin_counterfeiter

import (
	"go/ast"
	"go/token"
	"os"
	"regexp"
	"strings"
	"sync"

	"github.com/minauteur/go-generate-fast/src/plugins"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

const genRegex = `\/\/counterfeiter:generate\s+(.+)`

var genRe *regexp.Regexp

func init() {
	genRe = regexp.MustCompile(genRegex)
	plugins.RegisterPlugin(&CounterfeiterPlugin{})
}

func New() {
	plugins.RegisterPlugin(&CounterfeiterPlugin{})
}

type CounterfeiterPlugin struct {
	plugins.Plugin
}

func (p *CounterfeiterPlugin) Name() string {
	return "counterfeiter"
}

func (p *CounterfeiterPlugin) Matches(opts plugins.GenerateOpts) bool {
	return strings.Contains(strings.Join(opts.Words, ""), "counterfeiter")
}

func (p *CounterfeiterPlugin) ComputeInputOutputFiles(opts plugins.GenerateOpts) *plugins.InputOutputFiles {
	ioFiles := plugins.InputOutputFiles{}
	files := map[string]bool{} // for tracking unique input files to prevent dupes
	// First, look for additional //counterfeiter:generate directives
	bytes, err := os.ReadFile(opts.Path)
	if err != nil {
		zap.S().Errorf("counterfeiter: cannot read file %s: %s", opts.Path, err)
		return nil
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax,
		Dir:  opts.Dir(),
		Fset: &token.FileSet{},
	}
	packages, err := packages.Load(cfg)
	if err != nil {
		zap.S().Errorf("counterfeiter: cannot load packages in %s: %s", opts.Dir(), err)
		return nil
	}
	ioFiles.OutputPatterns = []string{packages[0].Name + "fakes/*.go"}

	// then, look for the generate directives
	interfaces := getInterfaceNames(opts, bytes)
	wg := &sync.WaitGroup{}
	fCh := make(chan string)
	wg.Add(len(interfaces))
	zap.S().Debugf("counterfeiter: found %d interfaces to generate: %v", len(interfaces), interfaces)
	for _, iFace := range interfaces {
		go func(inFiles []*ast.File, iFace string, wg *sync.WaitGroup, fCh chan<- string) {
			defer wg.Done()
			// once we have the interface name, we can load the package directory and look for the file where it is declared
			for _, file := range inFiles {
				for _, decl := range file.Decls {
					if isInterface(decl, iFace) {
						interfaceDeclaringFile := cfg.Fset.Position(decl.Pos()).Filename
						fCh <- interfaceDeclaringFile
						zap.S().Debugf("counterfeiter: found interface %s in %s", iFace, interfaceDeclaringFile)
						return
					}
				}
			}

			zap.S().Errorf("counterfeiter: cannot find interface %s in any file in %s", iFace, opts.Dir())

		}(packages[0].Syntax, iFace, wg, fCh)
	}
	go func(w *sync.WaitGroup, fCh chan string) {
		w.Wait()
		close(fCh)
	}(wg, fCh)
	for filename := range fCh {
		files[filename] = true
	}
	for file := range files {
		ioFiles.InputFiles = append(ioFiles.InputFiles, file)
	}
	zap.S().Debugf("counterfeiter: found %d input files and %d output patterns", len(ioFiles.InputFiles), len(ioFiles.OutputPatterns))
	zap.S().Debugf("counterfeiter: input files: %s", strings.Join(ioFiles.InputFiles, ", "))
	zap.S().Debugf("counterfeiter: output patterns: %s", strings.Join(ioFiles.OutputPatterns, ", "))
	return &ioFiles
}

func getInterfaceNames(opts plugins.GenerateOpts, b []byte) []string {
	results := []string{}
	matches := genRe.FindAllStringSubmatch(string(b), -1)
	for _, match := range matches {
		if len(match) != 2 {
			zap.S().Errorf("counterfeiter: invalid directive in %s: %s", opts.Path, strings.Join(match, " "))
			return nil
		}
		packageAndInterface := strings.Split(match[1], " ")
		if len(packageAndInterface) != 2 {
			zap.S().Errorf("counterfeiter: invalid directive in %s: %s", opts.Path, strings.Join(packageAndInterface, " "))
			return nil
		}
		pkg := packageAndInterface[0]
		if pkg != "." {
			zap.S().Errorf("counterfeiter: package %s is not supported, only current package (.) is allowed", pkg)
			return nil
		}
		results = append(results, packageAndInterface[1])
	}
	return results
}

func isInterface(astDec ast.Decl, identifier string) bool {
	if decl, ok := astDec.(*ast.GenDecl); ok {
		for _, spec := range decl.Specs {
			if t, ok := spec.(*ast.TypeSpec); ok {
				if _, ok := t.Type.(*ast.InterfaceType); ok {
					if t.Name.Name == identifier {
						return true
					}
				}
			}
		}
	}
	return false
}
