package credentials

import "errors"

var ErrNotImplemented = errors.New("credentials: not implemented")

func EnvVar(providerName, credential string) string {
	_ = providerName
	_ = credential
	return ""
}

func Save(path, providerName, credential, secret string) error {
	_ = path
	_ = providerName
	_ = credential
	_ = secret
	return ErrNotImplemented
}

func Load(path, providerName, credential string) (secret string, ok bool, err error) {
	_ = path
	_ = providerName
	_ = credential
	return "", false, ErrNotImplemented
}
