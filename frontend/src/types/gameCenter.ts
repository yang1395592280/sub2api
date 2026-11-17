export type GameCenterClaimStatus = 'pending' | 'claimable' | 'claimed'
export type GameCenterOpenMode = 'embed' | 'standalone' | 'dual'

export interface GameCenterClaimBatch {
  batch_key: string
  claim_time?: string
  status: GameCenterClaimStatus
  points_amount: number
  claim_date?: string
}

export interface GameCenterExchangeConfig {
  balance_to_points_enabled: boolean
  points_to_balance_enabled: boolean
  balance_to_points_rate: number
  points_to_balance_rate: number
  min_balance_amount?: number
  min_points_amount?: number
}

export interface GameCenterCatalog {
  game_key: string
  name: string
  subtitle?: string
  description?: string
  cover_image?: string
  default_open_mode: GameCenterOpenMode
  supports_embed?: boolean
  supports_standalone?: boolean
}

export interface GameCenterLedgerItem {
  id: number
  user_id?: number
  email?: string
  username?: string
  entry_type: string
  delta_points: number
  points_after?: number
  reason?: string
  created_at?: string
}

export interface GameCenterPointsLeaderboardItem {
  rank: number
  user_id: number
  email: string
  username: string
  points: number
}

export interface GameCenterOverview {
  points: number
  claim_batches: GameCenterClaimBatch[]
  exchange: GameCenterExchangeConfig
  catalogs: GameCenterCatalog[]
  recent_ledger: GameCenterLedgerItem[]
}

export interface ExchangeBalanceToPointsRequest {
  amount: number
}

export interface ExchangePointsToBalanceRequest {
  points: number
}
