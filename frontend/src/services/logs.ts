import { Call } from '@wailsio/runtime'

export type RequestLog = {
  id: number
  platform: string
  model: string
  provider: string
  http_code: number
  input_tokens: number
  output_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  reasoning_tokens: number
  is_stream?: boolean | number
  duration_sec?: number
  created_at: string
  total_cost?: number
  input_cost?: number
  output_cost?: number
  cache_create_cost?: number
  cache_read_cost?: number
  ephemeral_5m_cost?: number
  ephemeral_1h_cost?: number
  has_pricing?: boolean
}

export type RequestLogPayload = {
  log_id: number
  has_payload: boolean
  request_headers: string
  request_body: string
  response_headers: string
  response_body: string
  upstream_request_body: string
  upstream_response_body: string
  request_truncated: boolean
  response_truncated: boolean
  created_at: string
}

type RequestLogQuery = {
  platform?: string
  provider?: string
  limit?: number
}

export const fetchRequestLogs = async (query: RequestLogQuery = {}): Promise<RequestLog[]> => {
  const platform = query.platform ?? ''
  const provider = query.provider ?? ''
  const limit = query.limit ?? 100
  return Call.ByName('codeswitch/services.LogService.ListRequestLogs', platform, provider, limit)
}

export const fetchLogProviders = async (platform = ''): Promise<string[]> => {
  return Call.ByName('codeswitch/services.LogService.ListProviders', platform)
}

export const fetchRequestLogPayload = async (id: number): Promise<RequestLogPayload> => {
  return Call.ByName('codeswitch/services.LogService.GetRequestLogPayload', id)
}

export type LogStatsSeries = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  total_cost: number
}

export type LogStats = {
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
  cost_input: number
  cost_output: number
  cost_cache_create: number
  cost_cache_read: number
  series: LogStatsSeries[]
}

export const fetchLogStats = async (platform = ''): Promise<LogStats> => {
  return Call.ByName('codeswitch/services.LogService.StatsSince', platform)
}

export type ProviderDailyStat = {
  provider: string
  total_requests: number
  successful_requests: number
  failed_requests: number
  success_rate: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  cache_create_tokens: number
  cache_read_tokens: number
  cost_total: number
}

export const fetchProviderDailyStats = async (
  platform = '',
): Promise<ProviderDailyStat[]> => {
  return Call.ByName('codeswitch/services.LogService.ProviderDailyStats', platform)
}

export type HeatmapStat = {
  day: string
  total_requests: number
  input_tokens: number
  output_tokens: number
  reasoning_tokens: number
  total_cost: number
}

export const fetchHeatmapStats = async (days: number): Promise<HeatmapStat[]> => {
  const range = Number.isFinite(days) && days > 0 ? Math.floor(days) : 30
  return Call.ByName('codeswitch/services.LogService.HeatmapStats', range)
}
