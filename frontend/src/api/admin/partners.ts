/**
 * Admin Partners API endpoints
 */

import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export interface Partner {
  id: number
  partner_name: string
  email: string
  phone: string
  referral_code: string
  pending_points: number
  withdrawn_points: number
  notes: string
  status: string
  created_at: string
  updated_at: string
  referred_users_count: number
}

export interface PartnerCommission {
  id: number
  partner_id: number
  referred_user_id: number
  referred_user_email: string
  points: number
  source_cost: number
  created_at: string
}

export interface CreatePartnerRequest {
  partner_name: string
  email?: string
  phone?: string
  referral_code?: string
  notes?: string
}

export interface UpdatePartnerRequest {
  partner_name?: string
  email?: string
  phone?: string
  notes?: string
  status?: string
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: string
    search?: string
  }
): Promise<BasePaginationResponse<Partner>> {
  const { data } = await apiClient.get<BasePaginationResponse<Partner>>('/admin/partners', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getById(id: number): Promise<Partner> {
  const { data } = await apiClient.get<Partner>(`/admin/partners/${id}`)
  return data
}

export async function create(request: CreatePartnerRequest): Promise<Partner> {
  const { data } = await apiClient.post<Partner>('/admin/partners', request)
  return data
}

export async function update(id: number, request: UpdatePartnerRequest): Promise<Partner> {
  const { data } = await apiClient.put<Partner>(`/admin/partners/${id}`, request)
  return data
}

export async function deletePartner(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(`/admin/partners/${id}`)
  return data
}

export async function withdrawPoints(id: number, amount: number): Promise<Partner> {
  const { data } = await apiClient.post<Partner>(`/admin/partners/${id}/withdraw`, { amount })
  return data
}

export async function listCommissions(
  id: number,
  page: number = 1,
  pageSize: number = 20
): Promise<BasePaginationResponse<PartnerCommission>> {
  const { data } = await apiClient.get<BasePaginationResponse<PartnerCommission>>(
    `/admin/partners/${id}/commissions`,
    { params: { page, page_size: pageSize } }
  )
  return data
}

const partnersAPI = {
  list,
  getById,
  create,
  update,
  delete: deletePartner,
  withdrawPoints,
  listCommissions
}

export default partnersAPI
