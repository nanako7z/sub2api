/**
 * Referral System API endpoints
 * Handles referral stats, code generation, referred users, and commission records
 */

import apiClient from './client'

export interface ReferralStats {
  referral_code: string
  total_referred: number
  total_commission: number
}

export interface ReferredUser {
  user_id: number
  email: string
  username: string
  total_commission: number
  joined_at: string
}

export interface CommissionRecord {
  id: number
  referrer_id: number
  referred_user_id: number
  amount: number
  source_cost: number
  commission_rate: number
  created_at: string
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export function getReferralStats() {
  return apiClient.get<ReferralStats>('/referral/stats')
}

export function generateReferralCode() {
  return apiClient.post<{ referral_code: string }>('/referral/code')
}

export function getReferredUsers(params: { page: number; page_size: number }) {
  return apiClient.get<PaginatedResponse<ReferredUser>>('/referral/users', { params })
}

export function getReferralCommissions(params: { page: number; page_size: number }) {
  return apiClient.get<PaginatedResponse<CommissionRecord>>('/referral/commissions', { params })
}
