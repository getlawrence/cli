package types

type PackageType string

const (
	PackageTypeAPI             PackageType = "API"
	PackageTypeSDK             PackageType = "SDK"
	PackageTypeInstrumentation PackageType = "Instrumentation"
)

type PackageLanguage string

const (
	PackageLanguageGo     PackageLanguage = "go"
	PackageLanguagePython PackageLanguage = "python"
)

type Package struct {
	Type          PackageType
	Language      PackageLanguage
	Name          string
	Version       string
	Documentation string
	URL           string
	Repository    string
}
