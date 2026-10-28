import { describe, expect, it } from 'vitest'
import {
  adaptServerQualityBuckets,
  buildQualityMonitor,
  type QualityMonitorRun,
  type QualityVerdict,
} from '../qualityMonitor'

const now = Date.parse('2026-10-28T04:00:30Z')
const options = { policy_id: 11, scope_hash: 'current-scope', now_ms: now, interval_seconds: 1800 }
const windowStart = now - 86400000

function run(id: number, verdict: QualityVerdict | null = 'passed', overrides: Partial<QualityMonitorRun> = {}): QualityMonitorRun {
  return {
    id,
    policy_id: 11,
    scope_hash: 'current-scope',
    status: 'completed',
    verdict,
    queued_at: new Date(now - 120000).toISOString(),
    started_at: new Date(now - 60000).toISOString(),
    ...overrides,
  }
}

describe('quality monitor adapted from manxue-ai', () => {
  it('shows no accuracy or coverage without any samples', () => {
    const { summary, timeline } = buildQualityMonitor([], options)
    expect(summary.total).toBe(0)
    expect(summary.pass_rate).toBeNull()
    expect(summary.answer_coverage).toBeNull()
    expect(timeline).toHaveLength(48)
    expect(timeline.every(bucket => bucket.status === 'empty' && bucket.run_ids.length === 0)).toBe(true)
  })

  it('separates answer accuracy from failed calls instead of treating errors as wrong answers', () => {
    const runs = Array.from({ length: 60 }, (_, index) => run(index + 1, index < 3 ? 'passed' : index === 3 ? 'incorrect' : 'error'))
    const { summary } = buildQualityMonitor(runs, options)
    expect(summary).toMatchObject({ total: 60, completed: 60, passed: 3, incorrect: 1, errors: 56, valid_answers: 4, pass_rate: 75 })
    expect(summary.answer_coverage).toBeCloseTo(4 / 60 * 100)
  })

  it('does not mix another policy, an old routing scope, or a duplicate refreshed run', () => {
    const { summary, timeline } = buildQualityMonitor([
      run(1, 'incorrect'),
      run(2, 'incorrect', { policy_id: 12 }),
      run(3, 'incorrect', { scope_hash: 'previous-scope' }),
      run(1, 'passed'),
    ], options)
    expect(summary).toMatchObject({ total: 1, passed: 1, incorrect: 0, pass_rate: 100 })
    expect(timeline.flatMap(bucket => bucket.run_ids)).toEqual([1])
  })

  it('covers exactly the rolling 24 hours including the edge partial slots', () => {
    const { summary, timeline } = buildQualityMonitor([
      run(1, 'passed', { started_at: new Date(windowStart).toISOString() }),
      run(2, 'passed', { started_at: new Date(windowStart - 1).toISOString() }),
      run(3, 'passed', { started_at: new Date(now).toISOString() }),
      run(4, 'passed', { started_at: new Date(now + 1).toISOString() }),
      run(5, 'passed', { started_at: 'invalid-time' }),
    ], options)
    expect(summary.total).toBe(2)
    expect(timeline[0].start_ms).toBe(windowStart)
    expect(timeline[0].run_ids).toEqual([1])
    expect(timeline.at(-1)?.end_ms).toBe(now)
    expect(timeline.at(-1)?.run_ids).toEqual([3])
  })

  it('keeps adjacent slots disjoint and bounds a final short slot', () => {
    const intervalSeconds = 700
    const { timeline } = buildQualityMonitor([
      run(1, 'passed', { started_at: new Date(windowStart + intervalSeconds * 1000 - 1).toISOString() }),
      run(2, 'incorrect', { started_at: new Date(windowStart + intervalSeconds * 1000).toISOString() }),
    ], { ...options, interval_seconds: intervalSeconds })
    expect(timeline[0].run_ids).toEqual([1])
    expect(timeline[1].run_ids).toEqual([2])
    expect(timeline.at(-1)?.end_ms).toBe(now)
    expect(timeline.at(-1)!.end_ms - timeline.at(-1)!.start_ms).toBeLessThan(intervalSeconds * 1000)
  })

  it('preserves all drill-down IDs when a running or failed task wins a slot color', () => {
    const { summary, timeline } = buildQualityMonitor([
      run(3, 'passed'),
      run(2, 'error'),
      run(1, null, { status: 'running' }),
    ], options)
    expect(timeline.at(-1)).toMatchObject({ status: 'running', run_ids: [1, 2, 3] })
    expect(summary).toMatchObject({ running: 1, completed: 2, passed: 1, errors: 1 })
  })

  it('keeps unknown and non-answer terminal states out of the valid-answer denominator', () => {
    const { summary } = buildQualityMonitor([
      run(1, 'unknown'),
      run(2, null),
      run(3, null, { status: 'queued', started_at: null }),
      run(4, null, { status: 'cancelled', started_at: null }),
      run(5, null, { status: 'interrupted' }),
      run(6, null, { status: 'skipped', started_at: null }),
      run(7, 'passed', { status: 'running' }),
    ], options)
    expect(summary).toMatchObject({ total: 7, unknown: 2, queued: 1, cancelled: 1, interrupted: 1, skipped: 1, running: 1, valid_answers: 0, pass_rate: null, answer_coverage: 0 })
  })

  it('rejects unbounded intervals or missing scope before allocating the timeline', () => {
    for (const interval of [0, 60, 86401, Infinity, NaN]) {
      expect(() => buildQualityMonitor([], { ...options, interval_seconds: interval })).toThrow(RangeError)
    }
    expect(() => buildQualityMonitor([], { ...options, scope_hash: '' })).toThrow(RangeError)
    expect(() => buildQualityMonitor([], { ...options, now_ms: NaN })).toThrow(RangeError)
  })

  it('adapts complete server buckets without treating bounded samples as full run counts', () => {
    const result = adaptServerQualityBuckets({
      complete: true,
      buckets: [{ index: 0, start_at: '2026-10-27T04:00:00Z', end_at: '2026-10-27T04:30:00Z', status: 'incorrect', total: 12, sample_run_ids: [1, 2, 3, 4, 5], has_more: true }],
    })
    expect(result).toEqual([{ index: 0, start_ms: Date.parse('2026-10-27T04:00:00Z'), end_ms: Date.parse('2026-10-27T04:30:00Z'), status: 'incorrect', run_ids: [1, 2, 3, 4, 5], total: 12, has_more: true }])
  })

  it('rejects incomplete or malformed server aggregates instead of rendering false empty slots', () => {
    const bucket = { index: 0, start_at: '2026-10-27T04:00:00Z', end_at: '2026-10-27T04:30:00Z', status: 'passed', total: 1, sample_run_ids: [1], has_more: false }
    expect(() => adaptServerQualityBuckets({ complete: false, buckets: [bucket] })).toThrow(RangeError)
    expect(() => adaptServerQualityBuckets({ complete: true, buckets: [{ ...bucket, status: 'healthy' }] })).toThrow(RangeError)
    expect(() => adaptServerQualityBuckets({ complete: true, buckets: [{ ...bucket, sample_run_ids: [1, 2, 3, 4, 5, 6] }] })).toThrow(RangeError)
  })
})
