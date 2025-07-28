package plugin_counterfeiter

import (
	"go/ast"
	"go/token"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/minauteur/go-generate-fast/src/plugins"
	"go.uber.org/zap"
	"golang.org/x/tools/go/packages"
)

const genRegex = `\/\/counterfeiter:generate\s+(.+)`
const pkgRegex = `^package\s+(\w+)`

var genRe, pkgRe *regexp.Regexp

func init() {
	genRe = regexp.MustCompile(genRegex)
	pkgRe = regexp.MustCompile(pkgRegex)
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
	globs := map[string]bool{} // for tracking unique output globs to prevent dupes
	// First, look for additional //counterfeiter:generate directives
	bytes, err := os.ReadFile(opts.Path)
	if err != nil {
		zap.S().Errorf("counterfeiter: cannot read file %s: %s", opts.Path, err)
		return nil
	}

	// then, look for the generate directives
	matches := genRe.FindAllStringSubmatch(string(bytes), -1)

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
		iFace := packageAndInterface[1]

		// once we have the interface name, we can load the package directory and look for the file where it is declared
		cfg := &packages.Config{
			Mode: packages.LoadAllSyntax,
			Dir:  opts.Dir(),
			Fset: &token.FileSet{},
		}
		packages, err := packages.Load(cfg)
		if err != nil {
			zap.S().Errorf("counterfeiter: cannot load packages in %s: %s", opts.Dir(), err)
			return nil
		}
		globs[packages[0].Name+"fakes/*.go"] = true
		foundInterfaceDecl := false
	outer:
		for _, file := range packages[0].Syntax {
			for _, decl := range file.Decls {
				if ok := ast.FilterDecl(decl, func(s string) bool {
					return s == iFace
				}); ok {
					interfaceDeclaringFile := cfg.Fset.Position(decl.Pos()).Filename
					zap.S().Debugf("counterfeiter: found interface %s in %s", iFace, path.Join(opts.Dir(), interfaceDeclaringFile))
					files[interfaceDeclaringFile] = true
					foundInterfaceDecl = true
					break outer
				}
			}
		}
		if !foundInterfaceDecl {
			zap.S().Errorf("counterfeiter: cannot find interface %s in any file in %s", iFace, opts.Dir())
			return nil
		}

	}
	for filename := range files {
		ioFiles.InputFiles = append(ioFiles.InputFiles, filename)
	}
	for pattern := range globs {
		ioFiles.OutputPatterns = append(ioFiles.OutputPatterns, pattern)
	}
	zap.S().Debugf("counterfeiter: found %d input files and %d output patterns", len(ioFiles.InputFiles), len(ioFiles.OutputPatterns))
	zap.S().Debugf("counterfeiter: input files: %s", strings.Join(ioFiles.InputFiles, ", "))
	zap.S().Debugf("counterfeiter: output patterns: %s", strings.Join(ioFiles.OutputPatterns, ", "))
	return &ioFiles
}
