package engine

import (
	"context"
	"errors"

	"github.com/camel-creatives/embeddings-api/internal/domain"
)

// ErrNoEngine is returned when no engine covers an input.
var ErrNoEngine = errors.New("no engine supports the input")

// Router implements domain.Embedder and dispatches each input to an engine
// whose Supports covers all of its parts. Results are reassembled in order.
type Router struct {
	engines []domain.Embedder
}

// New builds a router from the given engines.
func New(engines ...domain.Embedder) *Router {
	return &Router{engines: engines}
}

// Supports reports whether any engine handles k.
func (r *Router) Supports(k domain.PartKind) bool {
	for _, e := range r.engines {
		if e.Supports(k) {
			return true
		}
	}
	return false
}

// Ready checks every engine.
func (r *Router) Ready(ctx context.Context) error {
	if len(r.engines) == 0 {
		return domain.ErrEngineUnavail
	}
	for _, e := range r.engines {
		if err := e.Ready(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Embed routes inputs, groups them per engine, and merges the vectors.
func (r *Router) Embed(ctx context.Context, in []domain.Input) (domain.EmbedResult, error) {
	groups := map[int][]int{}
	order := make([]int, 0, len(r.engines))

	for i, item := range in {
		gi := r.pick(item)
		if gi < 0 {
			return domain.EmbedResult{}, domain.ErrUnsupportedIn.WithMessage("No engine supports this input")
		}
		if _, ok := groups[gi]; !ok {
			order = append(order, gi)
		}
		groups[gi] = append(groups[gi], i)
	}

	vectors := make([][]float32, len(in))
	var usage domain.Usage

	for _, gi := range order {
		idxs := groups[gi]
		sub := make([]domain.Input, len(idxs))
		for j, ii := range idxs {
			sub[j] = in[ii]
		}
		res, err := r.engines[gi].Embed(ctx, sub)
		if err != nil {
			return domain.EmbedResult{}, err
		}
		if len(res.Vectors) != len(sub) {
			return domain.EmbedResult{}, errors.New("engine returned wrong vector count")
		}
		for j, ii := range idxs {
			vectors[ii] = res.Vectors[j]
		}
		usage.PromptTokens += res.Usage.PromptTokens
		usage.TotalTokens += res.Usage.TotalTokens
		usage.Images += res.Usage.Images
		usage.AudioSeconds += res.Usage.AudioSeconds
		usage.VideoSeconds += res.Usage.VideoSeconds
	}

	return domain.EmbedResult{Vectors: vectors, Usage: usage}, nil
}

// pick returns the index of the first engine that supports every part.
func (r *Router) pick(in domain.Input) int {
	for i, e := range r.engines {
		ok := true
		for _, p := range in.Parts {
			if !e.Supports(p.Kind) {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return -1
}
