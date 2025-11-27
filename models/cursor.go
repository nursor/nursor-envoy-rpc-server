package models

type AccountInfo struct {
	ID              int    `json:"id"`
	Email           string `json:"email"`
	Name            string `json:"name"`
	Password        string `json:"password"`
	CursorID        string `json:"cursor_id"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	AccessToken     string `json:"access_token"`
	SubID           string `json:"sub_id"`
	RefreshToken    string `json:"refresh_token"`
	MembershipType  string `json:"membership_type"`
	CacheEmail      bool   `json:"cache_email"`
	UniqueCppUserID string `json:"unique_cpp_user_id"`
	ClientKey       string `json:"client_key"`
	DispatchOrder   int    `json:"dispatch_order"`
	Description     string `json:"description"`
	Status          string `json:"status"`
	ExpiresAt       *int64 `json:"expires_at"`
	CreatedAt       int64  `json:"created_at"`
	UpdatedAt       int64  `json:"updated_at"`
	Usage           int    `json:"usage"`
	DetailUsage     int    `json:"detail_usage"`
	UsageLimit      int    `json:"usage_limit"`
}
