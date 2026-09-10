import { apiClient } from '../client'

export type AdminPackageStatus = 'draft' | 'pending_review' | 'approved' | 'rejected' | 'taken_down'

export interface AdminPackage {
  id: number
  name: string
  description: string
  version: string
  status: AdminPackageStatus
  file_url: string
  storage_key: string
  size_bytes: number
  checksum: string
  plan_bindings: string[]
  permission_bindings: string[]
  download_count: number
  last_downloaded_at?: string
  created_by?: number
  created_at: string
  updated_at: string
  reviewed_by?: number
  reviewed_at?: string
  review_note: string
  takedown_reason: string
}

export interface SaveAdminPackageRequest {
  name: string
  description?: string
  version?: string
  status?: AdminPackageStatus
  file_url?: string
  storage_key?: string
  size_bytes?: number
  checksum?: string
  plan_bindings?: string[]
  permission_bindings?: string[]
}

export interface PackageStorageConfig {
  driver: string
  reuse_backup_s3: boolean
  base_path: string
  public_base_url: string
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  secret_access_key_configured?: boolean
  prefix: string
  presign_expiry_hours: number
  force_path_style: boolean
  updated_by?: number
  updated_at?: string
}

export interface PackageCloudKeyStatus {
  configured: boolean
  key_id: string
  algorithm: string
  source: string
  public_key: string
  updated_by?: number
  updated_at?: string
}

export interface PackageDownloadAudit {
  id: number
  package_id: number
  package_name: string
  actor_user_id?: number
  action: string
  client_ip: string
  user_agent: string
  created_at: string
}

export async function list(filters?: { status?: string; search?: string }): Promise<AdminPackage[]> {
  const { data } = await apiClient.get<AdminPackage[]>('/admin/packages', { params: filters })
  return data
}

export async function get(id: number): Promise<AdminPackage> {
  const { data } = await apiClient.get<AdminPackage>(`/admin/packages/${id}`)
  return data
}

export async function create(request: SaveAdminPackageRequest): Promise<AdminPackage> {
  const { data } = await apiClient.post<AdminPackage>('/admin/packages', request)
  return data
}

export async function update(id: number, request: SaveAdminPackageRequest): Promise<AdminPackage> {
  const { data } = await apiClient.put<AdminPackage>(`/admin/packages/${id}`, request)
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/packages/${id}`)
}

export async function updateBindings(
  id: number,
  request: Pick<SaveAdminPackageRequest, 'plan_bindings' | 'permission_bindings'>
): Promise<AdminPackage> {
  const { data } = await apiClient.put<AdminPackage>(`/admin/packages/${id}/bindings`, request)
  return data
}

export async function review(id: number, status: 'approved' | 'rejected', note?: string): Promise<AdminPackage> {
  const { data } = await apiClient.post<AdminPackage>(`/admin/packages/${id}/review`, { status, note })
  return data
}

export async function takeDown(id: number, reason?: string): Promise<AdminPackage> {
  const { data } = await apiClient.post<AdminPackage>(`/admin/packages/${id}/takedown`, { reason })
  return data
}

export async function listDownloadAudits(filters?: { package_id?: number; limit?: number }): Promise<PackageDownloadAudit[]> {
  const { data } = await apiClient.get<PackageDownloadAudit[]>('/admin/packages/audits', { params: filters })
  return data
}

export async function recordDownloadAudit(id: number): Promise<PackageDownloadAudit> {
  const { data } = await apiClient.post<PackageDownloadAudit>(`/admin/packages/${id}/download-audits`)
  return data
}

export async function getStorage(): Promise<PackageStorageConfig> {
  const { data } = await apiClient.get<PackageStorageConfig>('/admin/packages/storage')
  return data
}

export async function updateStorage(config: PackageStorageConfig): Promise<PackageStorageConfig> {
  const { data } = await apiClient.put<PackageStorageConfig>('/admin/packages/storage', config)
  return data
}

export async function getCloudKey(): Promise<PackageCloudKeyStatus> {
  const { data } = await apiClient.get<PackageCloudKeyStatus>('/admin/packages/cloud-key')
  return data
}

export async function generateCloudKey(): Promise<PackageCloudKeyStatus> {
  const { data } = await apiClient.post<PackageCloudKeyStatus>('/admin/packages/cloud-key/generate')
  return data
}

export default {
  list,
  get,
  create,
  update,
  remove,
  updateBindings,
  review,
  takeDown,
  listDownloadAudits,
  recordDownloadAudit,
  getStorage,
  updateStorage,
  getCloudKey,
  generateCloudKey
}
