package plugin_counterfeiter

import (
	"os"
	"regexp"
	"strings"

	"github.com/minauteur/go-generate-fast/src/plugins"
	"go.uber.org/zap"
)

const genRegex = `\/\/counterfeiter:generate\s+(.+)`
const packageRegex = `(?m)^package\s+([a-zA-Z_][a-zA-Z0-9_]*)`
const inRegex = `(?m)^//\s*([a-zA-Z0-9_\-]+\.go)\b`

var genRe, pkgRe, inRe *regexp.Regexp

func init() {
	genRe = regexp.MustCompile(genRegex)
	pkgRe = regexp.MustCompile(packageRegex)
	inRe = regexp.MustCompile(inRegex)
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

	bytes, err := os.ReadFile(opts.Path)
	if err != nil {
		zap.S().Errorf("counterfeiter: cannot read file %s: %s", opts.Path, err)
		return nil
	}

	pkg := pkgRe.Find(bytes)
	zap.S().Debug("counterfeiter: found package name in file: ", string(pkg))
	trimPackage := strings.TrimPrefix(string(pkg), "package ")
	ioFiles.OutputPatterns = []string{trimPackage + "fakes/*.go"}

	inputs := inRe.FindAllStringSubmatch(string(bytes), -1)
	for _, input := range inputs {
		zap.S().Debugf("counterfeiter: found input file: %s", input[1])
		ioFiles.InputFiles = append(ioFiles.InputFiles, input[1])
	}

	zap.S().Debugf("counterfeiter: found %d input files and %d output patterns", len(ioFiles.InputFiles), len(ioFiles.OutputPatterns))
	zap.S().Debugf("counterfeiter: input files: %s", strings.Join(ioFiles.InputFiles, ", "))
	zap.S().Debugf("counterfeiter: output patterns: %s", strings.Join(ioFiles.OutputPatterns, ", "))
	return &ioFiles
}
