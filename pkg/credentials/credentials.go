package credentials

import "errors"

// ErrNotImplemented is what Save and Load return until a later phase
// implements them for real — every test case exercising either is
// expected to fail against this, except TestLoad's "malformed-file-line"
// case, which passes trivially because the stub already returns a
// non-nil error.
var ErrNotImplemented = errors.New("credentials: not implemented")

// EnvVar returns the environment variable pobear reads/writes a
// provider's credential from: POBEAR_PROVIDER_<NAME>_<CREDENTIAL>, both
// segments uppercased (PROVIDER_REFERENCE.md §5). Stubbed until a later
// phase implements it for real.
func EnvVar(providerName, credential string) string {
	_ = providerName
	_ = credential
	return ""
}

// Save stores secret for providerName/credential at path, so a later
// Load call (in this or another process) finds it. path points at a
// file in the on-disk store format: one KEY=VALUE line per credential,
// KEY being exactly what EnvVar returns for that provider/credential.
// An existing line for the same key is updated in place; a new key is
// appended; every other line's content and order is preserved. The file
// is written with 0o600 permissions, since it holds plaintext secrets.
// Stubbed until a later phase implements it for real.
func Save(path, providerName, credential, secret string) error {
	_ = path
	_ = providerName
	_ = credential
	_ = secret
	return ErrNotImplemented
}

// Load returns the stored secret for providerName/credential: the
// environment variable (EnvVar) if set, otherwise the on-disk store at
// path. ok is false if neither has it — that's not itself an error. err
// is non-nil only if path exists and is malformed (a non-blank line with
// no "="). Stubbed until a later phase implements it for real.
func Load(path, providerName, credential string) (secret string, ok bool, err error) {
	_ = path
	_ = providerName
	_ = credential
	return "", false, ErrNotImplemented
}
