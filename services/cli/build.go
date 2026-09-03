package cli

import (
	"github.com/pobear/pobear/pkg/jobspec"
	"github.com/pobear/pobear/services/cli/internal/command"
	"github.com/pobear/pobear/services/cli/internal/providerator"
	"github.com/pobear/pobear/services/core"
)

func DefaultCredentialsPath() (string, error) {
	return providerator.DefaultCredentialPath()
}

func New(providersDir string, coreService *core.Service, credentialsPath string, prompt func(label string) (string, error)) *Service {
	providers := providerator.New(providersDir)
	runner := command.NewJobRunner(coreService)

	commands := map[string]command.Command{
		"version":   &command.Version{},
		"providers": &command.Providers{Providers: providers},
		"auth":      &command.Auth{Providers: providers, Prompt: prompt, Path: credentialsPath},
		"scrape":    &command.Scrape{Runner: runner, Providers: providers},
		"crawl":     &command.Crawl{Runner: runner, Providers: providers},
		"run":       &command.Run{Parser: jobspecParser{}, Runner: runner},
	}

	return NewService(commands)
}

type jobspecParser struct{}

func (jobspecParser) Parse(data []byte, mode jobspec.Mode) (*jobspec.Spec, error) {
	return jobspec.Parse(data, mode)
}
