package main

import (
	"os"

	"github.com/minauteur/go-generate-fast/src/core/config"
	"github.com/minauteur/go-generate-fast/src/core/generate/base"
	"github.com/minauteur/go-generate-fast/src/core/generate/generate"
	"github.com/minauteur/go-generate-fast/src/logger"
	"github.com/minauteur/go-generate-fast/src/plugin_factory"
	"go.uber.org/zap"
)

func main() {
	config.Init()
	logger.Init()
	plugin_factory.Init()

	zap.S().Debug("Starting")

	args := os.Args[1:]

	generate.RunGenerate(args)

	zap.S().Debug("End")

	base.Exit()
}
