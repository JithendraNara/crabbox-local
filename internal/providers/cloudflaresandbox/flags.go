package cloudflaresandbox

import (
	"flag"
	"strings"
)

type cloudflareSandboxFlagValues struct {
	APIURL        *string
	Token         *string
	Workdir       *string
	LegacyAPIURL  *string
	LegacyToken   *string
	LegacyWorkdir *string
}

func RegisterCloudflareSandboxProviderFlags(fs *flag.FlagSet, defaults Config) any {
	return cloudflareSandboxFlagValues{
		APIURL:        fs.String("cf-containers-url", defaults.CloudflareSandbox.APIURL, "CF Containers runner API URL"),
		Token:         fs.String("cf-containers-token", "", "CF Containers runner bearer token"),
		Workdir:       fs.String("cf-containers-workdir", defaults.CloudflareSandbox.Workdir, "Absolute working directory inside the CF Containers workspace"),
		LegacyAPIURL:  fs.String("cloudflare-sandbox-url", defaults.CloudflareSandbox.APIURL, "legacy alias for --cf-containers-url"),
		LegacyToken:   fs.String("cloudflare-sandbox-token", "", "legacy alias for --cf-containers-token"),
		LegacyWorkdir: fs.String("cloudflare-sandbox-workdir", defaults.CloudflareSandbox.Workdir, "legacy alias for --cf-containers-workdir"),
	}
}

func ApplyCloudflareSandboxProviderFlags(cfg *Config, fs *flag.FlagSet, values any) error {
	if isCloudflareContainersProviderName(cfg.Provider) {
		if flagWasSet(fs, "class") {
			return exit(2, "--class is not supported for provider=%s", providerName)
		}
		if flagWasSet(fs, "type") {
			return exit(2, "--type is not supported for provider=%s", providerName)
		}
	}
	v, ok := values.(cloudflareSandboxFlagValues)
	if !ok {
		return nil
	}
	if flagWasSet(fs, "cloudflare-sandbox-url") {
		cfg.CloudflareSandbox.APIURL = *v.LegacyAPIURL
	}
	if flagWasSet(fs, "cloudflare-sandbox-token") {
		cfg.CloudflareSandbox.Token = *v.LegacyToken
	}
	if flagWasSet(fs, "cloudflare-sandbox-workdir") {
		cfg.CloudflareSandbox.Workdir = *v.LegacyWorkdir
	}
	if flagWasSet(fs, "cf-containers-url") {
		cfg.CloudflareSandbox.APIURL = *v.APIURL
	}
	if flagWasSet(fs, "cf-containers-token") {
		cfg.CloudflareSandbox.Token = *v.Token
	}
	if flagWasSet(fs, "cf-containers-workdir") {
		cfg.CloudflareSandbox.Workdir = *v.Workdir
	}
	return nil
}

func isCloudflareContainersProviderName(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case providerName, "cloudflare-containers", "cloudflare", legacyProviderName, "cf-sandbox":
		return true
	default:
		return false
	}
}
