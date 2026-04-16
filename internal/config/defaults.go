package config

// Default configuration values
const (
	// DefaultDomain is the default domain for scdev projects
	// This is a wildcard domain that resolves to 127.0.0.1
	DefaultDomain = "scalecommerce.site"

	// GlobalConfigFilename is the filename for the global config
	// Distinct from project config.yaml to avoid confusion
	GlobalConfigFilename = "global-config.yaml"
)

// Shared service image versions
const (
	// RouterImage is the default Traefik router image
	RouterImage = "traefik:v3.6"

	// MailImage is the default Mailpit image
	MailImage = "axllent/mailpit:latest"

	// DBUIImage is the default Adminer image for database management
	DBUIImage = "adminer:latest"

	// RedisInsightsImage is the default Redis Insights image
	RedisInsightsImage = "redis/redisinsight:latest"

	// ObservabilityImage is the default OpenObserve image
	ObservabilityImage = "public.ecr.aws/zinclabs/openobserve:latest"

	// TestImage is the lightweight image used for integration tests
	// Keep in sync with testdata/projects/*/config.yaml fixtures
	TestImage = "alpine:latest"
)

// Plugin versions
const (
	// StatiqPluginModule is the Go module path for the Statiq plugin
	StatiqPluginModule = "github.com/hhftechnology/statiq"

	// StatiqPluginVersion is the version of the Statiq plugin
	StatiqPluginVersion = "v1.0.1"
)

// Tool versions and download URLs
const (
	// MkcertVersion is the version of mkcert to download
	MkcertVersion = "v1.4.4"

	// MkcertURLTemplate is the download URL template for mkcert
	// Use with fmt.Sprintf(MkcertURLTemplate, MkcertVersion, MkcertVersion, runtime.GOOS, arch)
	// where arch is "amd64" or "arm64"
	MkcertURLTemplate = "https://github.com/FiloSottile/mkcert/releases/download/%s/mkcert-%s-%s-%s"

	// JustVersion is the version of just to download
	JustVersion = "1.49.0"

	// JustURLTemplate is the download URL template for just
	// Use with fmt.Sprintf(JustURLTemplate, JustVersion, arch, os)
	// where arch is "x86_64" or "aarch64", os is "apple-darwin" or "unknown-linux-musl"
	JustURLTemplate = "https://github.com/casey/just/releases/download/%s/just-%s-%s.tar.gz"

	// CtopVersion is the version of ctop to download
	CtopVersion = "0.7.7"

	// CtopURLTemplate is the download URL template for ctop
	// Use with fmt.Sprintf(CtopURLTemplate, CtopVersion, os, arch)
	// where os is "darwin" or "linux", arch is "amd64" or "arm64"
	CtopURLTemplate = "https://github.com/bcicen/ctop/releases/download/v%s/ctop-%s-%s"

	// MutagenVersion is the version of mutagen to download
	MutagenVersion = "0.18.1"

	// MutagenURLTemplate is the download URL template for mutagen
	// Use with fmt.Sprintf(MutagenURLTemplate, MutagenVersion, os, arch, MutagenVersion)
	// where os is "darwin" or "linux", arch is "amd64" or "arm64"
	// Example: https://github.com/mutagen-io/mutagen/releases/download/v0.18.1/mutagen_darwin_arm64_v0.18.1.tar.gz
	MutagenURLTemplate = "https://github.com/mutagen-io/mutagen/releases/download/v%s/mutagen_%s_%s_v%s.tar.gz"
)
