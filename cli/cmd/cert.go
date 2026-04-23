package cmd

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

// CertInfo holds the structured output for a certificate inspection.
type CertInfo struct {
	Subject     string   `json:"subject"            yaml:"subject"`
	Issuer      string   `json:"issuer"             yaml:"issuer"`
	SANs        []string `json:"sans,omitempty"     yaml:"sans,omitempty"`
	NotBefore   string   `json:"not_before"         yaml:"not_before"`
	NotAfter    string   `json:"not_after"          yaml:"not_after"`
	DaysLeft    int      `json:"days_left"          yaml:"days_left"`
	Expired     bool     `json:"expired"            yaml:"expired"`
	Fingerprint string   `json:"fingerprint_sha256" yaml:"fingerprint_sha256"`
	KeyAlgo     string   `json:"key_algorithm"      yaml:"key_algorithm"`
	Serial      string   `json:"serial"             yaml:"serial"`
}

// certInfoFromCert converts an x509.Certificate to CertInfo.
func certInfoFromCert(cert *x509.Certificate) CertInfo {
	now := time.Now()
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)

	sans := make([]string, 0, len(cert.DNSNames)+len(cert.IPAddresses))
	sans = append(sans, cert.DNSNames...)
	for _, ip := range cert.IPAddresses {
		sans = append(sans, ip.String())
	}

	fp := sha256.Sum256(cert.Raw)
	fingerprint := fmt.Sprintf("%x", fp[:])

	return CertInfo{
		Subject:     cert.Subject.String(),
		Issuer:      cert.Issuer.String(),
		SANs:        sans,
		NotBefore:   cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:    cert.NotAfter.UTC().Format(time.RFC3339),
		DaysLeft:    daysLeft,
		Expired:     now.After(cert.NotAfter),
		Fingerprint: fingerprint,
		KeyAlgo:     cert.PublicKeyAlgorithm.String(),
		Serial:      cert.SerialNumber.String(),
	}
}

// fetchTLSCerts connects to host(:port) and returns the peer certificates.
func fetchTLSCerts(host string, insecure bool) ([]*x509.Certificate, error) {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = host + ":443"
	}
	cfg := &tls.Config{
		InsecureSkipVerify: insecure, //#nosec G402 -- user-controlled flag
	}
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: 10 * time.Second},
		"tcp", host, cfg,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			// Ignore close errors: connection state is already captured below.
			_ = closeErr
		}
	}()
	return conn.ConnectionState().PeerCertificates, nil
}

// parsePEMCertFile reads a PEM file and returns all CERTIFICATE blocks.
func parsePEMCertFile(path string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- user-controlled cert file
	if err != nil {
		return nil, err
	}
	return parsePEMCertBytes(data)
}

// parsePEMCertBytes parses raw PEM data and returns all CERTIFICATE blocks.
func parsePEMCertBytes(data []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
	}
	return certs, nil
}

func emitCertOutput(cmd *cobra.Command, infos []CertInfo, format string) {
	switch format {
	case fmtJSON:
		out, err := json.MarshalIndent(infos, "", "  ")
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	case fmtYAML:
		out, err := yaml.Marshal(infos)
		exitWithError(cmd, err)
		outputBytes(cmd, out)
	default:
		for _, info := range infos {
			outputString(cmd, fmt.Sprintf(
				"Subject:     %s\nIssuer:      %s\nExpires:     %s (%d days left)\nExpired:     %v\nSANs:        %v\nFingerprint: %s",
				info.Subject, info.Issuer, info.NotAfter, info.DaysLeft, info.Expired,
				info.SANs, info.Fingerprint,
			))
		}
	}
}

var certCmd = &cobra.Command{
	Use:   "cert",
	Short: "Inspect TLS/X.509 certificates",
	Long: `Inspect TLS or X.509 PEM certificates.

Subcommands:
  inspect   Inspect a certificate (from host or PEM file)
  expiry    Show only expiry information`,
}

var certInspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect a TLS certificate and show structured details",
	Long: `Inspect a TLS certificate from a live host or a PEM file.

Source priority:
  1. --host host[:port]   fetch live certificate via TLS
  2. --file path          read PEM file
  3. stdin                read PEM from stdin

Examples:
  sdt cert inspect --host example.com
  sdt cert inspect --host example.com:8443 --format json
  sdt cert inspect --file cert.pem
  cat cert.pem | sdt cert inspect`,
	Run: func(cmd *cobra.Command, args []string) {
		host := getStringFlag(cmd, "host", false)
		filePath := getStringFlag(cmd, "file", false)
		insecure := getBoolFlag(cmd, "insecure", false)
		format := getFormat(cmd)

		var certs []*x509.Certificate
		var err error

		switch {
		case host != "":
			certs, err = fetchTLSCerts(host, insecure)
			exitWithError(cmd, err)
		case filePath != "":
			certs, err = parsePEMCertFile(filePath)
			exitWithError(cmd, err)
		default:
			pemData := getInputBytes(cmd, args)
			certs, err = parsePEMCertBytes(pemData)
			exitWithError(cmd, err)
		}

		if len(certs) == 0 {
			exitWithError(cmd, fmt.Errorf("no certificates found"))
			return
		}

		infos := make([]CertInfo, 0, len(certs))
		for _, c := range certs {
			infos = append(infos, certInfoFromCert(c))
		}
		emitCertOutput(cmd, infos, format)
	},
}

var certExpiryCmd = &cobra.Command{
	Use:   "expiry",
	Short: "Show certificate expiry information",
	Long: `Show only expiry date and days remaining for a certificate.

Examples:
  sdt cert expiry --host example.com
  sdt cert expiry --host example.com:443 --format json
  sdt cert expiry --file cert.pem`,
	Run: func(cmd *cobra.Command, args []string) {
		host := getStringFlag(cmd, "host", false)
		filePath := getStringFlag(cmd, "file", false)
		insecure := getBoolFlag(cmd, "insecure", false)
		format := getFormat(cmd)

		var certs []*x509.Certificate
		var err error

		switch {
		case host != "":
			certs, err = fetchTLSCerts(host, insecure)
			exitWithError(cmd, err)
		case filePath != "":
			certs, err = parsePEMCertFile(filePath)
			exitWithError(cmd, err)
		default:
			pemData := getInputBytes(cmd, args)
			certs, err = parsePEMCertBytes(pemData)
			exitWithError(cmd, err)
		}

		if len(certs) == 0 {
			exitWithError(cmd, fmt.Errorf("no certificates found"))
			return
		}

		cert := certs[0]
		now := time.Now()
		daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)
		expired := now.After(cert.NotAfter)

		type expiryResult struct {
			Host     string `json:"host,omitempty" yaml:"host,omitempty"`
			NotAfter string `json:"not_after"      yaml:"not_after"`
			DaysLeft int    `json:"days_left"      yaml:"days_left"`
			Expired  bool   `json:"expired"        yaml:"expired"`
		}
		result := expiryResult{
			Host:     host,
			NotAfter: cert.NotAfter.UTC().Format(time.RFC3339),
			DaysLeft: daysLeft,
			Expired:  expired,
		}

		switch format {
		case fmtJSON:
			out, merr := json.MarshalIndent(result, "", "  ")
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		case fmtYAML:
			out, merr := yaml.Marshal(result)
			exitWithError(cmd, merr)
			outputBytes(cmd, out)
		default:
			if expired {
				outputString(cmd, fmt.Sprintf("EXPIRED %s (%d days ago)",
					cert.NotAfter.UTC().Format(time.RFC3339), -daysLeft))
			} else {
				outputString(cmd, fmt.Sprintf("expires %s (%d days left)",
					cert.NotAfter.UTC().Format(time.RFC3339), daysLeft))
			}
		}
	},
}

func init() {
	certInspectCmd.Flags().String("host", "", "Host (or host:port) to fetch certificate from")
	certInspectCmd.Flags().String("file", "", "Path to PEM certificate file")
	certInspectCmd.Flags().Bool("insecure", false, "Skip TLS certificate verification")

	certExpiryCmd.Flags().String("host", "", "Host (or host:port) to fetch certificate from")
	certExpiryCmd.Flags().String("file", "", "Path to PEM certificate file")
	certExpiryCmd.Flags().Bool("insecure", false, "Skip TLS certificate verification")

	certCmd.AddCommand(certInspectCmd)
	certCmd.AddCommand(certExpiryCmd)
	rootCmd.AddCommand(certCmd)
}
