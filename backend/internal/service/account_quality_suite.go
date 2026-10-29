package service

import (
	"context"
	"fmt"
	"time"

	quality "github.com/Wei-Shaw/sub2api/internal/accountquality"
	"github.com/google/uuid"
)

func (p *accountQualityProvider) RendererConfigured() bool {
	return p.renderer != nil && p.renderer.Configured()
}

func (p *accountQualityProvider) Probe(ctx context.Context, run quality.Run, dispatch func(string) error) quality.Result {
	started := time.Now()
	candy := p.request(ctx, run, "candy", quality.CandyPromptFor(run.Snapshot.Condition), nil, dispatch)
	if run.Snapshot.Condition.BenchmarkID != "dual" {
		return candy
	}
	evidence := &quality.Evidence{Candy: candy.ResponseText}
	candyCheck := quality.Check(candy)
	var pelican, review *quality.CheckResult
	finish := func() quality.Result {
		r := quality.Combine(candyCheck, pelican, review, evidence)
		r.LatencyMS = time.Since(started).Milliseconds()
		return r
	}
	if ctx.Err() != nil {
		pelican = &quality.CheckResult{Verdict: "unknown", ReasonCode: "probe_timeout"}
		return finish()
	}
	nonce := uuid.NewString()[:8]
	generated := p.request(ctx, run, "pelican", fmt.Sprintf(quality.PelicanPrompt, nonce), nil, dispatch)
	evidence.Pelican = generated.ResponseText
	pelican = quality.Check(generated)
	if !generated.GenerationCompleted || generated.ResponseTruncated || generated.Verdict == "error" {
		return finish()
	}
	svg, structure := quality.InspectSVG(generated.ResponseText, nonce, true)
	evidence.SVG = svg
	pelican.Verdict, pelican.ReasonCode, pelican.Previewable = structure.Verdict, structure.ReasonCode, structure.Previewable
	if structure.ReasonCode != "review_pending" {
		return finish()
	}
	if run.Snapshot.Reviewer == nil {
		pelican.ReasonCode = "review_not_configured"
		return finish()
	}
	if !p.RendererConfigured() {
		pelican.ReasonCode = "renderer_not_configured"
		return finish()
	}
	frames, err := p.renderer.Render(ctx, svg)
	if err != nil {
		pelican.ReasonCode = "render_failed"
		return finish()
	}
	reviewer := run.Snapshot.Reviewer
	target, err := p.Resolve(ctx, reviewer.AccountID, reviewer.SourceGroupID, reviewer.Condition)
	if err != nil || !target.CanProbe {
		pelican.ReasonCode = "reviewer_unavailable"
		return finish()
	}
	// Each request owns and releases its concurrency slot. The same account may
	// be selected as reviewer without nesting a second slot acquisition.
	reviewRun := run
	reviewRun.AccountID = reviewer.AccountID
	reviewRun.Snapshot = quality.Policy{AccountID: reviewer.AccountID, SourceGroupID: reviewer.SourceGroupID, Condition: reviewer.Condition, Rules: run.Snapshot.Rules}
	reviewRun.Snapshot.SetTarget(target)
	reviewRun.ScopeHash = reviewRun.Snapshot.ScopeHash
	response := p.request(ctx, reviewRun, "review", quality.VisualReviewPrompt, frames, dispatch)
	evidence.Review = response.ResponseText
	review = quality.Check(response)
	if response.GenerationCompleted && !response.ResponseTruncated && response.Verdict != "error" {
		verdict := quality.ScoreReview(response.ResponseText, true)
		review.Verdict, review.ReasonCode, review.Checks = verdict.Verdict, verdict.ReasonCode, verdict.Checks
	}
	pelican.Verdict, pelican.ReasonCode, pelican.Checks = review.Verdict, review.ReasonCode, review.Checks
	return finish()
}
