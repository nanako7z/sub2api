package tlsfingerprint

import utls "github.com/refraction-networking/utls"

var defaultExtensionOrder = []uint16{65281, 0, 11, 10, 35, 16, 22, 23, 13, 43, 45, 51}

// BuiltInDefaultProfileSnapshot returns a materialized snapshot of the built-in
// default TLS fingerprint values used by the dialer fallback path.
func BuiltInDefaultProfileSnapshot() *Profile {
	return &Profile{
		Name:                "Built-in Default (Node.js 24.x)",
		Mode:                "npm",
		ShapeMode:           "npm",
		ShapeWindowSize:     64,
		ShapeWeights:        defaultTLSShapeWeights,
		ECHPayloadMode:      "npm",
		EnableGREASE:        false,
		CipherSuites:        append([]uint16(nil), defaultCipherSuites...),
		Curves:              curveIDsToUint16(defaultCurves),
		PointFormats:        append([]uint8(nil), defaultPointFormats...),
		SignatureAlgorithms: signatureSchemesToUint16(defaultSignatureAlgorithms),
		ALPNProtocols:       []string{"http/1.1"},
		SupportedVersions:   []uint16{utls.VersionTLS13, utls.VersionTLS12},
		KeyShareGroups:      []uint16{0x11ec, uint16(utls.X25519)},
		PSKModes:            []uint16{uint16(utls.PskModeDHE)},
		Extensions:          append([]uint16(nil), defaultExtensionOrder...),
	}
}

func curveIDsToUint16(in []utls.CurveID) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, v := range in {
		out = append(out, uint16(v))
	}
	return out
}

func signatureSchemesToUint16(in []utls.SignatureScheme) []uint16 {
	out := make([]uint16, 0, len(in))
	for _, v := range in {
		out = append(out, uint16(v))
	}
	return out
}
