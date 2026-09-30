package bff

import (
	"context"
	"errors"

	"github.com/JRAdams472/LENA2/internal/ai"
)

// AIAvailable reports whether the assistant is configured. Clients use it
// to hide AI entry points instead of erroring on first use.
func (r *Resolver) AIAvailable(ctx context.Context) (bool, error) {
	if _, err := userFromContext(ctx); err != nil {
		return false, err
	}
	return r.AIService != nil && r.AIService.Available(), nil
}

// AskAssistant runs one free-form assistant request. The model may use the
// read-only household tools; it can never write — actions always go through
// the normal mutations.
func (r *Resolver) AskAssistant(ctx context.Context, args struct{ Question string }) (*assistantAnswerResolver, error) {
	u, err := userFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if r.AIService == nil || !r.AIService.Available() {
		return nil, errUnavailablef("the AI assistant is not configured on this deployment")
	}
	if !r.aiLimiter().allow(u.UserID) {
		return nil, errUnavailablef("assistant rate limit reached — try again shortly")
	}
	ans, err := r.AIService.Ask(ctx, u.UserID, u.HouseholdID, args.Question)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrUnavailable):
			return nil, errUnavailablef("the AI assistant is not configured on this deployment")
		case errors.Is(err, ai.ErrInvalidQuestion):
			return nil, badInputf("question must be 1-2000 characters")
		default:
			return nil, err
		}
	}
	out := &assistantAnswerResolver{answer: ans.Text}
	for _, t := range ans.Tools {
		out.toolCalls = append(out.toolCalls, &assistantToolCallResolver{name: t.Name})
	}
	return out, nil
}

// aiLimiter returns the per-user assistant rate limiter, lazily built so
// Resolver literals in tests still work. LLM calls are expensive, so the
// burst is tighter than the upload limiter.
func (r *Resolver) aiLimiter() *userRateLimiter {
	r.ocrMu.Lock()
	defer r.ocrMu.Unlock()
	if r.aiCalls == nil {
		r.aiCalls = newUserRateLimiter(10)
	}
	return r.aiCalls
}

type assistantAnswerResolver struct {
	answer    string
	toolCalls []*assistantToolCallResolver
}

// Answer is the assistant's final text.
func (a *assistantAnswerResolver) Answer() string { return a.answer }

// ToolCalls lists the household-data lookups the assistant made, in order.
func (a *assistantAnswerResolver) ToolCalls() []*assistantToolCallResolver { return a.toolCalls }

type assistantToolCallResolver struct {
	name string
}

// Name is the tool the assistant invoked (e.g. "get_pantry_inventory").
func (t *assistantToolCallResolver) Name() string { return t.name }
