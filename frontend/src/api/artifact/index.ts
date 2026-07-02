import { get, post } from '../../utils/request'
import { getApiBaseUrl } from '@/utils/api-base'

export type ArtifactType = 'markdown' | 'report' | 'ppt' | 'chart'
export type ArtifactStatus = 'pending' | 'ready' | 'failed'
export type ArtifactSharingPolicy = 'private' | 'tenant' | 'explicit'
export type GenerationStatus = 'idle' | 'generating' | 'ready' | 'failed'

export interface Artifact {
  id: string
  tenant_id: number
  user_id: string
  session_id?: string
  type: ArtifactType
  status: ArtifactStatus
  title: string
  source_kb_id?: string
  source_knowledge_id?: string
  source_wiki_page_id?: string
  storage_uri?: string
  storage_type?: string
  mime_type?: string
  size_bytes?: number
  sharing_policy: ArtifactSharingPolicy
  allowed_user_ids?: string[]
  metadata?: Record<string, any>
  created_at: string
  updated_at: string
}

export interface CreateArtifactParams {
  type: ArtifactType | string
  title?: string
  session_id?: string
  source_kb_id?: string
  source_knowledge_id?: string
  source_wiki_page_id?: string
  sharing_policy?: ArtifactSharingPolicy
  allowed_user_ids?: string[]
  metadata?: Record<string, any>
}

export function createArtifact(params: CreateArtifactParams) {
  return post('/api/v1/artifacts', params) as unknown as Promise<{ success: boolean; data: Artifact }>
}

export function generateArtifact(id: string, prompt?: string) {
  return post(`/api/v1/artifacts/${encodeURIComponent(id)}/generate`, { prompt: prompt || '' }) as unknown as Promise<{ success: boolean; data: { id: string; status: string } }>
}

export function listArtifacts(page = 1, pageSize = 20) {
  return get(`/api/v1/artifacts?page=${page}&page_size=${pageSize}`) as unknown as Promise<{
    success: boolean
    data: { artifacts: Artifact[]; total: number; page: number; page_size: number }
  }>
}

export function listSessionArtifacts(sessionId: string) {
  return get(`/api/v1/sessions/${encodeURIComponent(sessionId)}/artifacts`) as unknown as Promise<{
    success: boolean
    data: { artifacts: Artifact[]; total: number }
  }>
}

export function getArtifact(id: string) {
  return get(`/api/v1/artifacts/${encodeURIComponent(id)}`) as unknown as Promise<{
    success: boolean
    data: Artifact
  }>
}

export function downloadArtifactURL(id: string): string {
  const base = getApiBaseUrl()
  const token = localStorage.getItem('xinwiki_token') || ''
  return `${base}/api/v1/artifacts/${encodeURIComponent(id)}/download?token=${encodeURIComponent(token)}`
}

export function buildPresignedPreviewURL(storageURI: string): string {
  if (!storageURI) return ''
  const base = getApiBaseUrl()
  return `${base}/api/v1/files/presigned-preview?file_path=${encodeURIComponent(storageURI)}`
}
