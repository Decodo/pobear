package credentials

import (
	"fmt"
	"os"
	"strings"
)

func EnvVar(providerName, credential string) string {
	return "POBEAR_PROVIDER_" + strings.ToUpper(providerName) + "_" + strings.ToUpper(credential)
}

func Save(path, providerName, credential, secret string) error {
	key := EnvVar(providerName, credential)
	line := key + "=" + secret

	var lines []string
	data, err := os.ReadFile(path) //nolint:gosec // path is the caller-supplied credentials file, not user-controlled input
	switch {
	case err == nil:
		if len(data) > 0 {
			lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
		}
	case os.IsNotExist(err):
	default:
		return err
	}

	found := false
	for i, l := range lines {
		if k, _, ok := strings.Cut(l, "="); ok && k == key {
			lines[i] = line
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, line)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600) //nolint:gosec // path is the caller-supplied credentials file, not user-controlled input
}

func Load(path, providerName, credential string) (secret string, ok bool, err error) {
	key := EnvVar(providerName, credential)
	if v, present := os.LookupEnv(key); present {
		return v, true, nil
	}

	data, err := os.ReadFile(path) //nolint:gosec // path is the caller-supplied credentials file, not user-controlled input
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	content := strings.TrimRight(string(data), "\n")
	if content == "" {
		return "", false, nil
	}
	for _, line := range strings.Split(content, "\n") {
		k, v, cut := strings.Cut(line, "=")
		if !cut {
			return "", false, fmt.Errorf("credentials: malformed line %q in %s", line, path)
		}
		if k == key {
			return v, true, nil
		}
	}
	return "", false, nil
}
