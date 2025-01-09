package auth

type ClientPermissionConfig struct {
	DisabledUserSignup   bool `json:"disabledUserSignup"`
	DisabledUserDeletion bool `json:"disabledUserDeletion"`
}
