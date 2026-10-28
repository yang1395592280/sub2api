/*
 * Adapted from manxue-ai app/web/app.js (renderTimeline, renderState) and
 * app/server.py (Monitor.state), revision 5da79674417dd0748682f09b7b7467efd1968ea8.
 * Source: https://github.com/w1196396546/manxue-ai
 * SPDX-License-Identifier: Apache-2.0
 * License and attribution: third_party/manxue-ai/upstream/LICENSE and
 * third_party/manxue-ai/NOTICE, relative to the sub2api repository root.
 * Modified for sub2api: typed inputs, policy/scope isolation, an exact rolling
 * window, run deduplication, and separate answer accuracy/coverage. This module
 * only prepares monitor data; it never changes account or scheduler state.
 */

export type QualityRunStatus =
  | 'queued'
  | 'running'
  | 'completed'
  | 'cancelled'
  | 'interrupted'
  | 'skipped'

export type QualityVerdict = 'passed' | 'incorrect' | 'unknown' | 'error'

export interface QualityMonitorRun {
  id: number
  policy_id: number
  scope_hash: string
  status: QualityRunStatus
  verdict: QualityVerdict | null
  queued_at: string
  started_at: string | null
}

export type QualityTimelineStatus = Exclude<QualityRunStatus, 'completed'> | QualityVerdict | 'empty'

export interface QualityTimelineBucket {
  index: number
  start_ms: number
  end_ms: number
  status: QualityTimelineStatus
  run_ids: number[]
}

export interface ServerQualityMonitorBucket {
  index: number
  start_at: string
  end_at: string
  status: string
  total: number
  sample_run_ids: number[]
  has_more: boolean
}

export interface ServerQualityMonitorBuckets {
  complete: boolean
  buckets: readonly ServerQualityMonitorBucket[]
}

export interface AdaptedServerTimelineBucket extends QualityTimelineBucket {
  total: number
  has_more: boolean
}

export interface QualityMonitorSummary {
  total: number
  completed: number
  passed: number
  incorrect: number
  unknown: number
  errors: number
  queued: number
  running: number
  cancelled: number
  interrupted: number
  skipped: number
  valid_answers: number
  pass_rate: number | null
  answer_coverage: number | null
}

export interface QualityMonitorOptions {
  policy_id: number
  scope_hash: string
  now_ms: number
  interval_seconds: number
}

const WINDOW_MS = 24 * 60 * 60 * 1000
const STATUS_PRIORITY: Record<QualityTimelineStatus, number> = {
  running: 9,
  queued: 8,
  error: 7,
  incorrect: 6,
  unknown: 5,
  interrupted: 4,
  cancelled: 3,
  skipped: 2,
  passed: 1,
  empty: 0,
}

function timelineStatus(run: QualityMonitorRun): QualityTimelineStatus {
  return run.status === 'completed' ? run.verdict ?? 'unknown' : run.status
}

/** Adapt the upstream monitor to one policy and evidence scope, never a mixed account pool. */
export function buildQualityMonitor(
  runs: readonly QualityMonitorRun[],
  options: QualityMonitorOptions,
): { summary: QualityMonitorSummary; timeline: QualityTimelineBucket[] } {
  if (!Number.isSafeInteger(options.policy_id) || options.policy_id <= 0 || !options.scope_hash.trim()) {
    throw new RangeError('A policy and evidence scope are required')
  }
  if (!Number.isFinite(options.now_ms) || !Number.isFinite(new Date(options.now_ms).getTime())) {
    throw new RangeError('A valid server time is required')
  }
  if (!Number.isInteger(options.interval_seconds) || options.interval_seconds < 300 || options.interval_seconds > 86400) {
    throw new RangeError('The interval must be between 300 and 86400 seconds')
  }

  const start = options.now_ms - WINDOW_MS
  const interval = options.interval_seconds * 1000
  const timeline: QualityTimelineBucket[] = Array.from(
    { length: Math.ceil(WINDOW_MS / interval) },
    (_, index) => ({
      index,
      start_ms: start + index * interval,
      end_ms: Math.min(start + (index + 1) * interval, options.now_ms),
      status: 'empty',
      run_ids: [],
    }),
  )
  const summary: QualityMonitorSummary = {
    total: 0,
    completed: 0,
    passed: 0,
    incorrect: 0,
    unknown: 0,
    errors: 0,
    queued: 0,
    running: 0,
    cancelled: 0,
    interrupted: 0,
    skipped: 0,
    valid_answers: 0,
    pass_rate: null,
    answer_coverage: null,
  }

  // Overlapping list pages or refreshed rows must not count the same run twice.
  const unique = new Map<number, QualityMonitorRun>()
  for (const run of runs) {
    if (run.policy_id === options.policy_id && run.scope_hash === options.scope_hash && Number.isSafeInteger(run.id) && run.id > 0) {
      unique.set(run.id, run)
    }
  }
  const visible = [...unique.values()]
    .map(run => ({ run, time: Date.parse(run.started_at ?? run.queued_at) }))
    .filter(({ time }) => Number.isFinite(time) && time >= start && time <= options.now_ms)
    .sort((a, b) => a.time - b.time || a.run.id - b.run.id)

  for (const { run, time } of visible) {
    summary.total++
    const status = timelineStatus(run)
    if (run.status === 'completed') {
      summary.completed++
      if (status === 'passed') summary.passed++
      else if (status === 'incorrect') summary.incorrect++
      else if (status === 'error') summary.errors++
      else summary.unknown++
    } else {
      summary[run.status]++
    }
    const bucket = timeline[Math.min(Math.floor((time - start) / interval), timeline.length - 1)]
    bucket.run_ids.push(run.id)
    if (STATUS_PRIORITY[status] > STATUS_PRIORITY[bucket.status]) bucket.status = status
  }

  summary.valid_answers = summary.passed + summary.incorrect
  summary.pass_rate = summary.valid_answers ? summary.passed / summary.valid_answers * 100 : null
  summary.answer_coverage = summary.total ? summary.valid_answers / summary.total * 100 : null
  return { summary, timeline }
}

/** Keep the server's complete counts while adapting its bounded samples for the UI timeline. */
export function adaptServerQualityBuckets(monitor: ServerQualityMonitorBuckets): AdaptedServerTimelineBucket[] {
  if (!monitor.complete) throw new RangeError('The server monitor must contain a complete aggregation')
  return monitor.buckets.map(bucket => {
    const start = Date.parse(bucket.start_at)
    const end = Date.parse(bucket.end_at)
    if (!Number.isInteger(bucket.index) || bucket.index < 0 || !Number.isFinite(start) || !Number.isFinite(end) || end < start) {
      throw new RangeError('The server monitor contains an invalid time bucket')
    }
    if (!Object.prototype.hasOwnProperty.call(STATUS_PRIORITY, bucket.status)) throw new RangeError('The server monitor contains an unknown status')
    if (!Number.isSafeInteger(bucket.total) || bucket.total < 0 || !Array.isArray(bucket.sample_run_ids) || bucket.sample_run_ids.length > 5 || bucket.sample_run_ids.some(id => !Number.isSafeInteger(id) || id <= 0) || typeof bucket.has_more !== 'boolean') {
      throw new RangeError('The server monitor contains invalid sample run IDs')
    }
    return {
      index: bucket.index,
      start_ms: start,
      end_ms: end,
      status: bucket.status as QualityTimelineStatus,
      run_ids: [...bucket.sample_run_ids],
      total: bucket.total,
      has_more: bucket.has_more,
    }
  })
}
