package drivers

// Caps описывает технологические возможности платформы.
type Caps struct {
	SupportsPSK         bool
	SupportsKeepalive   bool
	SupportsNAT         bool
	SupportsMultiRoute  bool
	SupportsObfuscation bool
	NeedsPackageInstall bool
}

// Capabilities возвращает матрицу возможностей для указанного типа платформы.
func Capabilities(platform string) Caps {
	switch platform {
	case "linux":
		return Caps{
			SupportsPSK:         true,
			SupportsKeepalive:   true,
			SupportsNAT:         true,
			SupportsMultiRoute:  true,
			SupportsObfuscation: true,
			NeedsPackageInstall: true,
		}
	case "mikrotik":
		return Caps{
			SupportsPSK:         true,
			SupportsKeepalive:   true,
			SupportsNAT:         true,
			SupportsMultiRoute:  true,
			SupportsObfuscation: false,
			NeedsPackageInstall: false,
		}
	case "openwrt":
		return Caps{
			SupportsPSK:         true,
			SupportsKeepalive:   true,
			SupportsNAT:         true,
			SupportsMultiRoute:  true,
			SupportsObfuscation: false,
			NeedsPackageInstall: true,
		}
	default:
		return Caps{}
	}
}
