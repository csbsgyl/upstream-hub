/**
 * API response shapes for upstream-hub backend.
 * Keep in sync with backend/internal/storage/*.go and backend/internal/api/*.go.
 */

export type ChannelType = "newapi" | "sub2api"

export type CredentialMode = "password" | "token"

export type NotificationChannelType =
  | "telegram"
  | "webhook"
  | "email"
  | "wecom"
  | "dingtalk"
  | "feishu"
  | "serverchan"

export type CaptchaProviderType =
  | "capsolver"
  | "2captcha"
  | "anticaptcha"
  | "yescaptcha"

export type MonitorJob = "login" | "balance" | "rates"

export type BalanceTrendRange = "24h" | "7d" | "30d"

export type NotificationEvent =
  | "balance_low"
  | "rate_changed"
  | "login_failed"
  | "captcha_failed"
  | "monitor_failed"

export interface Channel {
  id: number
  name: string
  type: ChannelType
  site_url: string
  username: string
  credential_mode: CredentialMode
  turnstile_enabled: boolean
  captcha_config_id?: number | null
  balance_threshold: number
  monitor_enabled: boolean
  last_balance?: number | null
  last_balance_at?: string | null
  last_error?: string
  health_score?: number
  health_status?: string
  created_at: string
  updated_at: string
}

export interface CaptchaConfig {
  id: number
  name: string
  type: CaptchaProviderType
  endpoint?: string
  extra?: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface RateSnapshot {
  id: number
  channel_id: number
  model_name: string
  description?: string
  ratio: number
  completion_ratio: number
  first_seen_at: string
  last_seen_at: string
}

export interface RateChangeLog {
  id: number
  channel_id: number
  model_name: string
  old_ratio: number | null
  new_ratio: number
  old_completion_ratio?: number | null
  new_completion_ratio?: number
  changed_at: string
}

export interface BalanceSnapshot {
  id: number
  channel_id: number
  balance: number
  sampled_at: string
}

export interface NotificationSubscription {
  channel_id: number
  mode: "all" | "groups"
  groups?: string[]
}

export interface NotificationChannel {
  id: number
  name: string
  type: NotificationChannelType
  enabled: boolean
  subscriptions?: string
  created_at: string
  updated_at: string
}

export interface NotificationLog {
  id: number
  channel_id: number
  event: NotificationEvent
  subject: string
  body: string
  success: boolean
  error_message?: string
  sent_at: string
}

export interface MonitorLog {
  id: number
  channel_id: number
  job: MonitorJob
  success: boolean
  error_message?: string
  duration_ms: number
  started_at: string
  finished_at: string
}

export interface DashboardLowest {
  channel_id: number
  name: string
  balance: number | null
}

export interface DashboardChannelStat {
  id: number
  name: string
  type: string
  monitor_enabled: boolean
  last_balance?: number | null
  last_error?: string
}

export interface DashboardSummary {
  total_channels: number
  active_channels: number
  failed_channels: number
  total_balance: number
  lowest_balance: DashboardLowest | null
  channels: DashboardChannelStat[]
  recent_rate_changes: RateChangeLog[]
  recent_notification_logs: NotificationLog[]
}

export interface BalanceTrendPoint {
  at?: string
  day?: string
  balance: number
}

export interface AuditLog {
  id: number
  actor: string
  action: string
  resource_type: string
  resource_id: number
  summary: string
  metadata?: string
  created_at: string
}

export interface BackupState {
  name: string
  source: string
  size: number
  updated_at: string
}

export interface OpsBackupResponse {
  backup: BackupState
  backups: BackupState[]
}

export interface OpsRetentionResult {
  monitor_logs_deleted: number
  balance_snapshots_deleted: number
  notification_logs_deleted: number
  monitor_logs_days: number
  balance_snapshots_days: number
  notification_logs_days: number
  ran_at: string
}

export interface OpsScanResult {
  ok: boolean
  started: boolean
  job: "sync" | "balances" | "rates"
  channels: number
  message: string
  started_at: string
}

export interface OpsStatus {
  database: string
  auth_enabled: boolean
  app_secret_ready: boolean
  scheduler: Record<string, unknown>
  notifications: Record<string, unknown>
  channels: Record<string, unknown>
  captchas: Record<string, unknown>
  backups: BackupState[]
  recent_audit_logs: Record<string, unknown>[]
  recent_monitor_logs: Record<string, unknown>[]
  recent_rate_changes: Record<string, unknown>[]
  recent_notification_logs: Record<string, unknown>[]
  generated_at: string
}
