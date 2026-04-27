import type { LuckyWheelPrizeConfig } from '@/types/luckyWheel'

const palette = ['#f4b942', '#4cc9a6', '#43a1ff', '#ff8a5b', '#7dd36f', '#ffd166', '#ff6b6b', '#64c1ff', '#9ad576']

export interface LuckyWheelSegment extends LuckyWheelPrizeConfig {
  center_deg: number
  span_deg: number
  color: string
  value_label: string
  marker_label: string
  radius_rem: number
  label_variant: 'micro' | 'compact' | 'regular'
}

export function buildLuckyWheelSegments(prizes: LuckyWheelPrizeConfig[]): LuckyWheelSegment[] {
  const count = prizes.length || 1
  const spanDeg = 360 / count
  return prizes.map((prize, index) => {
    const centerDeg = index * spanDeg + spanDeg / 2
    return {
      ...prize,
      span_deg: spanDeg,
      center_deg: centerDeg,
      color: palette[index % palette.length],
      value_label: buildLuckyWheelValueLabel(prize),
      marker_label: buildLuckyWheelMarkerLabel(prize),
      radius_rem: 8.5,
      label_variant: 'regular',
    }
  })
}

export function buildLuckyWheelGradient(prizes: LuckyWheelPrizeConfig[]): string {
  if (!prizes.length) return 'radial-gradient(circle, #e2e8f0, #cbd5e1)'
  const count = prizes.length
  const spanPercent = 100 / count
  const stops = prizes.map((_, index) => {
    const start = index * spanPercent
    const end = (index + 1) * spanPercent
    return `${palette[index % palette.length]} ${start}% ${end}%`
  })
  return `conic-gradient(from -90deg, ${stops.join(', ')})`
}

export function computeLuckyWheelRotation(prizes: LuckyWheelPrizeConfig[], prizeKey: string, extraTurns = 6): number {
  const segment = buildLuckyWheelSegments(prizes).find(item => item.key === prizeKey)
  if (!segment) return extraTurns * 360
  return extraTurns * 360 + (360 - segment.center_deg)
}

function buildLuckyWheelMarkerLabel(prize: LuckyWheelPrizeConfig): string {
  if (prize.type === 'thanks') return '谢'
  return prize.delta_points > 0 ? '奖' : '罚'
}

function buildLuckyWheelValueLabel(prize: LuckyWheelPrizeConfig): string {
  if (prize.type === 'thanks') return '谢谢'
  return `${prize.delta_points > 0 ? '+' : ''}${prize.delta_points}`
}
