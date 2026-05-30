import { Call } from '@wailsio/runtime'

export type AppSettings = {
  show_heatmap: boolean
  show_home_title: boolean
  auto_start: boolean
  relay_port: number
  capture_raw_logs: boolean
  raw_log_max_bytes: number
  claude_thinking_rectifier: boolean
}

const DEFAULT_SETTINGS: AppSettings = {
  show_heatmap: true,
  show_home_title: true,
  auto_start: false,
  relay_port: 18101,
  capture_raw_logs: false,
  raw_log_max_bytes: 262144,
  claude_thinking_rectifier: true,
}

export const fetchAppSettings = async (): Promise<AppSettings> => {
  const data = await Call.ByName('codeswitch/services.AppSettingsService.GetAppSettings')
  return { ...DEFAULT_SETTINGS, ...(data ?? {}) }
}

export const saveAppSettings = async (settings: AppSettings): Promise<AppSettings> => {
  return Call.ByName('codeswitch/services.AppSettingsService.SaveAppSettings', settings)
}
