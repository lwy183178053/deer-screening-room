export type Account = {
  id: number
  email: string
  is_admin: boolean
  enabled: boolean
  balance: number
  created_at: string
}

export type Studio = { id: number; name: string; video_count: number }

export type Video = {
  id: number
  studio_id: number
  studio_name: string
  title: string
  poster_url?: string
  duration_ms: number
  size_bytes: number
  bit_rate: number
  width: number
  height: number
  video_codec: string
  audio_codec: string
  compatibility: 'ready' | 'unsupported'
  published: boolean
  available: boolean
  unlocked: boolean
  can_play: boolean
  updated_at: string
}

export type VideoPage = {
  videos: Video[]
  page: number
  page_size: number
  total: number
}

export type UserPage = {
  users: Account[]
  page: number
  page_size: number
  total: number
  all_total: number
}

export type Commerce = {
  video_price: number
  redeem_notice: string
}

export type WalletEntry = { id: number; delta: number; kind: string; description: string; created_at: string }
export type NodeInfo = { id: number; name: string; wireguard_address?: string; provisioned?: boolean; revoked?: boolean; bundle_downloaded?: boolean; online: boolean; total_bytes: number; available_bytes: number; last_seen_at?: string; scan_status: 'unknown' | 'pending' | 'scanning' | 'ok' | 'error'; last_scan_at?: string; scan_error?: string }
export type RedeemCode = { id: number; credits: number; used: boolean; redeemed_by_email?: string; redeemed_at?: string; created_at: string }
export type RedeemCodeCounts = { all: number; used: number; unused: number }
export type RedeemCodePage = { codes: RedeemCode[]; page: number; page_size: number; total: number; counts: RedeemCodeCounts }
