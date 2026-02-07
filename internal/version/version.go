package version

const (
	Version     = "1.1.0"
	Name        = "MMCD Datalogger"
	Description = "Cross-platform ECU datalogging and diagnostics tool for 1G DSM (1990-1994 Mitsubishi Eclipse, Eagle Talon, Plymouth Laser)"
	Copyright   = "© 2026 Kevin Buckham & Claude (Anthropic)"
	Developers  = "Kevin Buckham & Claude (Anthropic)"
	License     = "GPL-2.0-or-later"
	Attribution = "Inspired by and thanks to the original MMCd PalmOS datalogger by Dmitry Yurtaev"
	URL         = "https://github.com/kbuckham/mmcd"
)

// Injected at build time via -ldflags
var (
	GitHash   = "dev"
	BuildTime = "unknown"
)

// FullVersion returns version string with git hash and build time.
func FullVersion() string {
	return Version + " (" + GitHash + ") built " + BuildTime
}
