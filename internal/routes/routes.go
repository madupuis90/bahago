package routes

// Route paths for all feature packages.
// Centralised here to break the import cycle between internal/ui and feature packages.
const (
	HomePath = "/home"
	ChatPath = "/chat"

	KingdomPath                = "/kingdom"
	KingdomResourcesPath       = "/kingdom/resources"
	KingdomResourcesLoadPath   = "/kingdom/resources/load"
	KingdomResourcesCreatePath = "/kingdom/resources/create"

	LoginPath              = "/login"
	RegisterPath           = "/register"
	VerifyPath             = "/verify"
	LogoutPath             = "/logout"
	ForgotPasswordPath     = "/forgot-password"
	ResetPasswordPath      = "/reset-password"
	ResendVerificationPath = "/resend-verification"
)
