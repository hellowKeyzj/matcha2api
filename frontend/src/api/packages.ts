import { apiClient } from './client'
import type { FetchOptions, PaginatedResponse } from '@/types'

export type ResourcePackageType = 'agent' | 'skill'
export type ResourcePackageVersionStatus = 'draft' | 'published' | 'disabled' | 'yanked'
export type ResourcePackageEntitlementStatus = 'active' | 'revoked' | 'expired'

export interface PackageMeteringBinding {
  id?: string
  type?: ResourcePackageType | string
  unit?: 'turn' | 'session' | string
  amount?: number
  currency?: string
}

export interface ResourcePackageVersion {
  packageId: string
  packageVersionId: string
  name: string
  displayName?: string
  packageType: ResourcePackageType | string
  version: string
  description?: string
  status: ResourcePackageVersionStatus | string
  entitlementStatus?: ResourcePackageEntitlementStatus | string
  downloadable: boolean
  meteringBinding?: PackageMeteringBinding
  downloadCount?: number
  createdAt?: string
  updatedAt?: string
}

export interface PackageListParams {
  page?: number
  page_size?: number
  search?: string
  packageType?: ResourcePackageType | string
}

export async function listMine(
  params?: PackageListParams,
  options?: FetchOptions,
): Promise<PaginatedResponse<ResourcePackageVersion>> {
  const { data } = await apiClient.get<PaginatedResponse<ResourcePackageVersion>>('/packages/mine', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function listMarket(
  params?: PackageListParams,
  options?: FetchOptions,
): Promise<PaginatedResponse<ResourcePackageVersion>> {
  const { data } = await apiClient.get<PaginatedResponse<ResourcePackageVersion>>('/packages/market', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function uploadPackage(file: File): Promise<ResourcePackageVersion> {
  const form = new FormData()
  form.append('package', file)

  const { data } = await apiClient.post<ResourcePackageVersion>('/packages/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
    timeout: 120000,
  })
  return data
}

export async function publishPackageVersion(packageVersionId: string): Promise<ResourcePackageVersion> {
  const { data } = await apiClient.post<ResourcePackageVersion>(`/packages/${packageVersionId}/publish`)
  return data
}

export async function downloadPackage(packageVersionId: string): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/packages/${packageVersionId}/download`, {
    responseType: 'blob',
  })
  return data
}

export const packagesAPI = {
  listMine,
  listMarket,
  uploadPackage,
  publishPackageVersion,
  downloadPackage,
}

export default packagesAPI
