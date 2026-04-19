package routes

// Route paths for all feature packages.
// Centralised here to break the import cycle between internal/layout and handler packages.
const (
	HomePath = "/home"
	ChatPath = "/chat"

	KingdomPath                  = "/kingdom"
	KingdomSetupPath             = "/kingdom/setup"
	KingdomCreatePath            = "/kingdom/create"
	KingdomAllocationPath        = "/kingdom/allocation"
	KingdomAllocationSavePath    = "/kingdom/allocation/save"
	KingdomRefreshPath           = "/kingdom/refresh"
	KingdomAllocationRefreshPath = "/kingdom/allocation/refresh"

	KingdomBuildingsPath         = "/kingdom/buildings"
	KingdomBuildingsRefreshPath  = "/kingdom/buildings/refresh"
	KingdomConstructionStartPath = "/kingdom/buildings/start"

	KingdomUnitsPath        = "/kingdom/units"
	KingdomUnitsRefreshPath = "/kingdom/units/refresh"
	KingdomUnitsTrainPath   = "/kingdom/units/train"

	KingdomMapPath = "/kingdom/map"

	LoginPath              = "/login"
	RegisterPath           = "/register"
	VerifyPath             = "/verify"
	LogoutPath             = "/logout"
	ForgotPasswordPath     = "/forgot-password"
	ResetPasswordPath      = "/reset-password"
	ResendVerificationPath = "/resend-verification"
)
