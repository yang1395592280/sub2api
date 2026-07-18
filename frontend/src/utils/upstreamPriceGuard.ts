type PriceGuardExtra = Record<string, unknown>

function getNumber(extra: PriceGuardExtra, key: string): number | null {
  const value = extra[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return null
}

function formatRate(value: number): string {
  return `${Number(value.toFixed(6)).toString()}x`
}

export function getUpstreamPriceGuardLabel(
  extra: PriceGuardExtra | null | undefined,
  currentPrice?: number | null
): string {
  const status = String(extra?.upstream_price_guard_status ?? '').toLowerCase()
  if (!status || status === 'ok') return ''

  const actual = extra ? getNumber(extra, 'upstream_price_guard_actual_multiplier') : null
  const max = extra ? getNumber(extra, 'upstream_price_guard_max_multiplier') : null

  if (status === 'blocked') {
    if (actual != null && max != null) {
      return `价格超限 ${formatRate(actual)} > ${formatRate(max)}`
    }
    if (actual != null) {
      return `价格超限 ${formatRate(actual)}`
    }
    if (max != null) {
      return `价格超限 > ${formatRate(max)}`
    }
    return '价格超限'
  }
  if (status === 'unsupported') {
    // 手动刷新余额会先更新 channel_price，价格保护快照可能暂时仍是旧的 unsupported。
    // 此时使用当前渠道价格修正文案，但不改变后端价格保护与调度状态。
    if (currentPrice != null && Number.isFinite(currentPrice) && currentPrice > 0) {
      if (max != null && max > 0 && currentPrice > max) {
        return `价格超限 ${formatRate(currentPrice)} > ${formatRate(max)}`
      }
      return ''
    }
    return '价格未知'
  }
  if (status === 'error') return '价格检查失败'
  return status
}
