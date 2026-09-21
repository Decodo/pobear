package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pobear/pobear/pkg/provider"
	"github.com/pobear/pobear/services/cli/internal/providerator"
)

type cssFieldPair struct {
	name, selector string
}

type fetchConfig struct {
	localConfigured bool
	profile         string
	proxyURL        string
	providerName    string
	templateName    string
	templateQuery   string
	providerOptions map[string]string
}

func newFetchConfig() fetchConfig {
	return fetchConfig{profile: "chrome"}
}

type extractConfig struct {
	extract   string
	cssFields []cssFieldPair
}

func newExtractConfig() extractConfig {
	return extractConfig{extract: "bare"}
}

const (
	addFieldName   = "addField"
	saveActionName = "saveAction"
	runActionName  = "runAction"
)

func cssFieldName(i int, part string) string {
	return "css." + part + "." + strconv.Itoa(i)
}

func localTierFields(c fetchConfig) []field {
	return []field{choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, c.profile)}
}

func proxyTierFields(c fetchConfig) []field {
	return []field{
		textField("proxyURL", "Proxy URL", "", c.proxyURL),
		choiceField("profile", "Profile", "(TLS fingerprint)", []string{"chrome", "firefox", "safari"}, c.profile),
	}
}

func providerTierFields(c fetchConfig, manifests []*provider.Manifest) []field {
	return providerAndOptionFields(c, manifests)
}

func tierRowField(name, label string, configured bool, summary string) field {
	value := "— not set"
	if configured {
		value = "✓ " + summary
	}
	return field{name: name, label: label, hint: "(enter to configure)", kind: fieldChoice, options: []string{value}, choice: 0}
}

func fetchTierRows(c fetchConfig) []field {
	providerSummary := c.providerName
	if providerSummary != "" && c.templateName != "" {
		providerSummary += " / " + c.templateName
	}
	return []field{
		tierRowField("localTier", "Local", c.localConfigured, c.profile),
		tierRowField("proxyTier", "Proxy", c.proxyURL != "", c.proxyURL),
		tierRowField("providerTier", "Provider", c.providerName != "", providerSummary),
	}
}

func ladderTiers(c fetchConfig) []string {
	var tiers []string
	if c.localConfigured {
		tiers = append(tiers, "local")
	}
	if c.proxyURL != "" {
		tiers = append(tiers, "proxy")
	}
	if c.providerName != "" {
		tiers = append(tiers, "provider")
	}
	return tiers
}

func resolveFetchUseAndLadder(c fetchConfig) (use string, ladder bool) {
	tiers := ladderTiers(c)
	if len(tiers) == 0 {
		return "", false
	}
	return tiers[0], len(tiers) > 1
}

func extractFields(c extractConfig) []field {
	extractField := choiceField("extract", "Extract", "(bare / raw / markdown / css fields)", []string{"bare", "raw", "markdown", "css fields"}, c.extract)
	if bareEasterEgg && c.extract == "bare" {
		extractField.displayOverride = "bear"
	}
	fields := []field{extractField}

	if c.extract == "css fields" {
		for i, pair := range c.cssFields {
			fields = append(fields,
				textField(cssFieldName(i, "name"), "  Field name", "(ctrl+d remove)", pair.name),
				textField(cssFieldName(i, "selector"), "    Selector", "", pair.selector),
			)
		}
		fields = append(fields, field{name: addFieldName, label: "  + add field", kind: fieldChoice, options: []string{""}})
	}
	return fields
}

func manifestNames(manifests []*provider.Manifest) []string {
	names := make([]string, len(manifests))
	for i, m := range manifests {
		names[i] = m.Name
	}
	return names
}

func effectiveChoice(current string, options []string) string {
	if current != "" || len(options) == 0 {
		return current
	}
	return options[0]
}

func providerAndOptionFields(c fetchConfig, manifests []*provider.Manifest) []field {
	names := manifestNames(manifests)
	displayNames := names
	if len(displayNames) == 0 {
		displayNames = []string{"(none configured)"}
	} else {
		displayNames = append([]string{""}, names...)
	}
	fields := []field{choiceField("provider", "Provider", "(type to search)", displayNames, c.providerName)}

	m, ok := providerator.Lookup(manifests, effectiveChoice(c.providerName, names))
	if !ok {
		return fields
	}
	return append(fields, providerFields(c, m)...)
}

func defaultTemplateName(m *provider.Manifest) string {
	for _, t := range m.Templates {
		if t.Default {
			return t.Name
		}
	}
	if len(m.Templates) == 0 {
		return ""
	}
	return m.Templates[0].Name
}

func effectiveTemplateName(m *provider.Manifest, current string) string {
	if len(m.Templates) < 2 || current != "" {
		return current
	}
	return defaultTemplateName(m)
}

func providerFields(c fetchConfig, m *provider.Manifest) []field {
	switch len(m.Templates) {
	case 0:
		return optionFields(m.Options, m.Name, c)
	case 1:
		opts, _ := providerator.ResolveTemplate(m, "")
		return optionFields(opts, m.Name, c)
	}

	templateNames := make([]string, len(m.Templates))
	for i, t := range m.Templates {
		templateNames[i] = t.Name
	}
	fields := []field{templateSearchField(templateNames, defaultTemplateName(m), c.templateQuery)}
	selected := effectiveTemplateName(m, c.templateName)
	if opts, err := providerator.ResolveTemplate(m, selected); err == nil {
		fields = append(fields, optionFields(opts, selected, c)...)
	}
	return fields
}

func optionFields(opts []provider.Option, hintSource string, c fetchConfig) []field {
	hint := "(" + hintSource + " option)"
	fields := make([]field, len(opts))
	for i, opt := range opts {
		fields[i] = optionField(opt, hint, optionValue(c, opt))
	}
	return fields
}

func optionFieldName(name string) string {
	return "opt." + name
}

func optionLabel(name string) string {
	label := strings.ReplaceAll(name, "_", " ")
	if label == "" {
		return label
	}
	return strings.ToUpper(label[:1]) + label[1:]
}

func optionValue(c fetchConfig, opt provider.Option) string {
	if v, ok := c.providerOptions[opt.Name]; ok {
		return v
	}
	if opt.Default != nil {
		return fmt.Sprint(opt.Default)
	}
	return ""
}

func optionField(opt provider.Option, hint, current string) field {
	name, label := optionFieldName(opt.Name), optionLabel(opt.Name)
	switch opt.Type {
	case "enum":
		return choiceField(name, label, hint, opt.Values, current)
	case "bool":
		return choiceField(name, label, hint, []string{"false", "true"}, current)
	case "int":
		min := 0
		if opt.Min != nil {
			min = *opt.Min
		}
		return numberField(name, label, hint, current, min, opt.Max)
	default:
		return textField(name, label, hint, current)
	}
}

func syncProviderOrTemplateChanged(fields []field, fc *fetchConfig) bool {
	for _, fld := range fields {
		if fld.name != "provider" {
			continue
		}
		if v := fld.value(); v != fc.providerName {
			fc.providerName = v
			fc.templateName = ""
			fc.templateQuery = ""
			fc.providerOptions = nil
			return true
		}
		break
	}

	for _, fld := range fields {
		if fld.name != "template" {
			continue
		}
		fc.templateQuery = fld.rawText()
		if v := fld.value(); v != fc.templateName {
			fc.templateName = v
			fc.providerOptions = nil
			return true
		}
		break
	}
	return false
}

func cssFieldIndex(name, prefix string) (int, bool) {
	i, err := strconv.Atoi(strings.TrimPrefix(name, prefix))
	return i, err == nil
}

func cssPairIndexFromFieldName(name string) (int, bool) {
	if i, ok := cssFieldIndex(name, "css.name."); ok {
		return i, true
	}
	return cssFieldIndex(name, "css.selector.")
}
