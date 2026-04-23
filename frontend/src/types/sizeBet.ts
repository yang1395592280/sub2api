export type SizeBetDirection = 'small' | 'mid' | 'big'
export type SizeBetPhase = 'betting' | 'closed' | 'maintenance'
export type SizeBetRoundStatus = 'open' | 'settled'
export type SizeBetStatus = 'placed' | 'won' | 'lost' | 'refunded'

export interface SizeBetCurrentRound {
  id: number
  round_no: number
  status: SizeBetRoundStatus
  starts_at: string
  bet_closes_at: string
  settles_at: string
  prob_small: number
  prob_mid: number
  prob_big: number
  odds_small: number
  odds_mid: number
  odds_big: number
  allowed_stakes: number[]
  server_seed_hash: string
  countdown_seconds: number
  bet_countdown_seconds: number
}

export interface SizeBetBet {
  id: number
  round_id: number
  direction: SizeBetDirection
  stake_amount: number
  payout_amount: number
  net_result_amount: number
  status: SizeBetStatus
  placed_at: string
  settled_at?: string | null
}

export interface SizeBetRoundSummary {
  id: number
  round_no: number
  status: SizeBetRoundStatus
  starts_at: string
  settles_at: string
  result_number?: number | null
  result_direction?: SizeBetDirection | '' | null
  server_seed_hash?: string
  server_seed?: string | null
}

export interface SizeBetCurrentRoundView {
  enabled: boolean
  phase: SizeBetPhase
  server_time: string
  round: SizeBetCurrentRound | null
  my_bet: SizeBetBet | null
  previous_round: SizeBetRoundSummary | null
}

export interface SizeBetRulesView {
  enabled: boolean
  round_duration_seconds: number
  bet_close_offset_seconds: number
  allowed_stakes: number[]
  probabilities: {
    small: number
    mid: number
    big: number
  }
  odds: {
    small: number
    mid: number
    big: number
  }
  rules_markdown: string
}

export interface PlaceSizeBetRequest {
  round_id: number
  direction: SizeBetDirection
  stake_amount: number
  idempotency_key: string
}
