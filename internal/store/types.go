package store

import "time"

var (
	ErrNotFound     = errorString("not found")
	ErrConflict     = errorString("conflict")
	ErrInsufficient = errorString("insufficient credits")
	ErrUnavailable  = errorString("unavailable")
	ErrCodeUsed     = errorString("redeem code already used")
	ErrCodeInvalid  = errorString("redeem code invalid")
	ErrForbidden    = errorString("forbidden")
)

type errorString string

func (e errorString) Error() string { return string(e) }

type User struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	IsAdmin   bool      `json:"is_admin"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type UserAuth struct {
	User
	PasswordHash string `json:"-"`
}

type Account struct {
	User
	Balance int64 `json:"balance"`
}

type UserPage struct {
	Users    []Account `json:"users"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int64     `json:"total"`
	AllTotal int64     `json:"all_total"`
}

type WalletEntry struct {
	ID          int64     `json:"id"`
	Delta       int64     `json:"delta"`
	Kind        string    `json:"kind"`
	ReferenceID string    `json:"reference_id"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type RedeemCode struct {
	ID              int64      `json:"id"`
	Credits         int64      `json:"credits"`
	Used            bool       `json:"used"`
	RedeemedByEmail *string    `json:"redeemed_by_email,omitempty"`
	RedeemedAt      *time.Time `json:"redeemed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type RedeemCodeCounts struct {
	All    int64 `json:"all"`
	Used   int64 `json:"used"`
	Unused int64 `json:"unused"`
}

type RedeemCodePage struct {
	Codes    []RedeemCode     `json:"codes"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
	Counts   RedeemCodeCounts `json:"counts"`
}

type Node struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	BaseURL          string     `json:"-"`
	WireGuardAddress string     `json:"wireguard_address,omitempty"`
	Provisioned      bool       `json:"provisioned"`
	Revoked          bool       `json:"revoked"`
	BundleDownloaded bool       `json:"bundle_downloaded"`
	Online           bool       `json:"online"`
	TotalBytes       int64      `json:"total_bytes"`
	AvailableBytes   int64      `json:"available_bytes"`
	LastSeenAt       *time.Time `json:"last_seen_at,omitempty"`
}

type ProvisionedNode struct {
	Node
	WireGuardPublicKey        string
	WireGuardPrivateKeySealed string
	APITokenSealed            string
	RelayTokenSealed          string
}

type Studio struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int64  `json:"video_count"`
}

type Video struct {
	ID            int64     `json:"id"`
	NodeID        int64     `json:"-"`
	NodeName      string    `json:"-"`
	NodeURL       string    `json:"-"`
	StudioID      int64     `json:"studio_id"`
	StudioName    string    `json:"studio_name"`
	MediaKey      string    `json:"-"`
	SourceTitle   string    `json:"-"`
	Title         string    `json:"title"`
	PosterKey     string    `json:"-"`
	PosterURL     string    `json:"poster_url"`
	DurationMS    int64     `json:"duration_ms"`
	SizeBytes     int64     `json:"size_bytes"`
	BitRate       int64     `json:"bit_rate"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	VideoCodec    string    `json:"video_codec"`
	AudioCodec    string    `json:"audio_codec"`
	Compatibility string    `json:"compatibility"`
	Published     bool      `json:"published"`
	Available     bool      `json:"available"`
	Unlocked      bool      `json:"unlocked"`
	CanPlay       bool      `json:"can_play"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type VideoPage struct {
	Videos   []Video `json:"videos"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int64   `json:"total"`
}

type MediaItem struct {
	MediaKey, Studio, Title, PosterKey, VideoCodec, AudioCodec, Compatibility string
	DurationMS, SizeBytes, BitRate                                            int64
	Width, Height                                                             int
}
