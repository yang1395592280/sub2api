const moneyFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
  useGrouping: false
})

// Display only: preserve the original precision in requests, billing and exports.
export function formatWalletAmount(value?: number | null): string {
  const amount = value ?? 0
  const formatted = moneyFormatter.format(Number.isFinite(amount) ? amount : 0)
  return formatted === '-0.00' ? '0.00' : formatted
}
