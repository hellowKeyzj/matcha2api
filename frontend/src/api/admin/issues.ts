import { apiClient } from '../client'
import type { BasePaginationResponse, FetchOptions } from '@/types'

export type DesktopIssueKind = 'bug' | 'feature' | 'experience'
export type DesktopIssueSeverity = 'blocker' | 'major' | 'minor'
export type DesktopIssueStatus = 'open' | 'in_progress' | 'resolved' | 'closed'

export interface DesktopIssue {
  id: number
  reporter_user_id?: number
  reporter_email: string
  reporter_username: string
  title: string
  description: string
  kind: DesktopIssueKind
  severity: DesktopIssueSeverity
  status: DesktopIssueStatus
  labels: string[]
  assignee_user_id?: number
  assignee_name: string
  fix_version: string
  app_version: string
  client_os: string
  client_arch: string
  client_route: string
  client_locale: string
  client_meta: Record<string, unknown>
  has_diagnostics: boolean
  image_count: number
  video_count: number
  comment_count: number
  created_at: string
  updated_at: string
  resolved_at?: string
  closed_at?: string
}

export interface DesktopIssueAttachment {
  id: number
  issue_id: number
  category: 'image' | 'video' | 'diagnostics'
  file_name: string
  mime_type: string
  size_bytes: number
  sha256: string
  created_at: string
}

export interface DesktopIssueEvent {
  id: number
  issue_id: number
  event_type:
    | 'created'
    | 'note'
    | 'status_changed'
    | 'labels_changed'
    | 'assignee_changed'
    | 'severity_changed'
    | 'fix_version_changed'
  actor_user_id?: number
  actor_name: string
  body: string
  metadata: Record<string, unknown>
  created_at: string
}

export interface DesktopIssueDetail {
  issue: DesktopIssue
  attachments: DesktopIssueAttachment[]
  events: DesktopIssueEvent[]
  reporter_issue_count: number
}

export interface DesktopIssueStatusCounts {
  all: number
  open: number
  in_progress: number
  resolved: number
  closed: number
}

export interface DesktopIssueListResponse extends BasePaginationResponse<DesktopIssue> {
  counts: DesktopIssueStatusCounts
}

export interface DesktopIssueListQuery {
  page?: number
  page_size?: number
  status?: string
  kind?: string
  severity?: string
  has_diagnostics?: boolean
  reporter?: string
  search?: string
}

export interface DesktopIssueManifestEntry {
  name: string
  size_bytes: number
}

export async function list(params: DesktopIssueListQuery, options?: FetchOptions): Promise<DesktopIssueListResponse> {
  const { data } = await apiClient.get<DesktopIssueListResponse>('/admin/issues', { params, signal: options?.signal })
  return data
}

export async function getById(id: number, options?: FetchOptions): Promise<DesktopIssueDetail> {
  const { data } = await apiClient.get<DesktopIssueDetail>(`/admin/issues/${id}`, { signal: options?.signal })
  return data
}

export async function update(
  id: number,
  payload: {
    status?: DesktopIssueStatus
    severity?: DesktopIssueSeverity
    labels?: string[]
    assignee_name?: string
    fix_version?: string
  },
  options?: FetchOptions
): Promise<DesktopIssueDetail> {
  const { data } = await apiClient.patch<DesktopIssueDetail>(`/admin/issues/${id}`, payload, { signal: options?.signal })
  return data
}

export async function addNote(
  id: number,
  payload: {
    body: string
    status?: DesktopIssueStatus
    fix_version?: string
  },
  options?: FetchOptions
): Promise<DesktopIssueDetail> {
  const { data } = await apiClient.post<DesktopIssueDetail>(`/admin/issues/${id}/notes`, payload, { signal: options?.signal })
  return data
}

export async function batchUpdateStatus(
  ids: number[],
  status: DesktopIssueStatus,
  options?: FetchOptions
): Promise<{ updated: number }> {
  const { data } = await apiClient.post<{ updated: number }>('/admin/issues/batch-status', { ids, status }, { signal: options?.signal })
  return data
}

export async function fetchAttachmentBlob(
  issueId: number,
  attachmentId: number,
  options?: FetchOptions & { download?: boolean }
): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/admin/issues/${issueId}/attachments/${attachmentId}`, {
    params: options?.download ? { download: '1' } : undefined,
    signal: options?.signal,
    responseType: 'blob',
    timeout: 120000
  })
  return data
}

export async function getDiagnosticsManifest(
  issueId: number,
  attachmentId: number,
  options?: FetchOptions
): Promise<{ entries: DesktopIssueManifestEntry[] }> {
  const { data } = await apiClient.get<{ entries: DesktopIssueManifestEntry[] }>(
    `/admin/issues/${issueId}/attachments/${attachmentId}/manifest`, { signal: options?.signal }
  )
  return data
}

export default {
  list,
  getById,
  update,
  addNote,
  batchUpdateStatus,
  fetchAttachmentBlob,
  getDiagnosticsManifest
}
