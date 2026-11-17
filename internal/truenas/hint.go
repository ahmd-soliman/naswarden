package truenas

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"syscall"

	"github.com/gorilla/websocket"
)

// Hint turns a failure to reach TrueNAS into the likely fix, worded for both
// the TrueNAS catalog app (form fields) and a compose setup (variables). The
// error itself, which names the address, is shown next to it. "" when there
// is nothing more specific to say than the error.
func Hint(err error) string {
	var dnsErr *net.DNSError
	var certErr *tls.CertificateVerificationError
	var unknownAuth x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var recordErr tls.RecordHeaderError

	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrKeyRejected):
		return "Check the API key, and that its user has the Readonly Admin role (Credentials → Users → the user → Access → View API Keys)."
	case errors.Is(err, syscall.ECONNREFUSED):
		return "Nothing answers on that address and port. Check the TrueNAS HTTPS port (System → General Settings → GUI): " +
			"the app's port setting, or the port in TRUENAS_HOST."
	case errors.As(err, &dnsErr):
		return "The TrueNAS host name does not resolve. Use the TrueNAS IP address instead."
	case errors.As(err, &certErr), errors.As(err, &unknownAuth), errors.As(err, &hostnameErr):
		return "The TrueNAS certificate is not trusted (TrueNAS's own certificate is self-signed). " +
			"Skip certificate verification (TRUENAS_INSECURE_TLS=true), or use a certificate this host trusts."
	case errors.As(err, &recordErr), errors.Is(err, websocket.ErrBadHandshake):
		return "TrueNAS answered, but not as expected: the port may be the HTTP port while TLS is on, or the other way round. " +
			"Use the HTTPS port with TLS on (TRUENAS_TLS=true)."
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return "TrueNAS did not answer in time. Check the address, and that no firewall blocks the port."
	}
	return ""
}
