package routes

// Route paths for all feature packages.
// Centralised here to break the import cycle between internal/ui and feature packages.
const (
	HomePath            = "/home"
	ChatPath            = "/chat"
	ResourcesPath       = "/resources"
	ResourcesLoadPath   = "/resources/load"
	ResourcesCreatePath = "/resources/create"
	RealmPath           = "/realm"

	LoginPath              = "/login"
	RegisterPath           = "/register"
	VerifyPath             = "/verify"
	LogoutPath             = "/logout"
	ForgotPasswordPath     = "/forgot-password"
	ResetPasswordPath      = "/reset-password"
	ResendVerificationPath = "/resend-verification"
)
