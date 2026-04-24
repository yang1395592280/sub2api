/**
 * API Client for Sub2API Backend
 * Central export point for all API modules
 */

// Re-export the HTTP client
export { apiClient } from './client'

// Auth API
export { authAPI, isTotp2FARequired, type LoginResponse } from './auth'

// User APIs
export { keysAPI } from './keys'
export { usageAPI } from './usage'
export { userAPI } from './user'
export { redeemAPI, type RedeemHistoryItem } from './redeem'
export {
  checkinAPI,
  type CheckinStatus,
  type CheckinResponse,
  type CheckinRecordSummary,
  type CheckinTodayRecord
} from './checkin'
export { paymentAPI } from './payment'
export { userGroupsAPI } from './groups'
export { totpAPI } from './totp'
export { default as announcementsAPI } from './announcements'
export { sizeBetAPI } from './sizeBet'
export { windsurfAccountsAPI } from './windsurfAccounts'

// Admin APIs
export { adminAPI } from './admin'

// Default export
export { default } from './client'
