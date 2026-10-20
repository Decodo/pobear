package extractors

import (
	"context"

	md "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/Decodo/pobear/pkg/jobspec"
)

type Markdown struct{}

func (Markdown) Run(ctx context.Context, body []byte, cfg jobspec.Stage) ([]Result, error) {
	out, err := md.ConvertString(string(body))
	if err != nil {
		return nil, err
	}
	return []Result{{Key: "markdown", Value: out}}, nil
}
