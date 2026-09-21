package tiers

type Result struct {
	Status   int
	Body     []byte
	FinalURL string
	Tier     string
}
