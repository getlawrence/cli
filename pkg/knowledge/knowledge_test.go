package knowledge

import (
	"fmt"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// StorageInterface defines the interface that Knowledge needs
type StorageInterface interface {
	Close() error
	GetComponentsLight(query storage.Query) *storage.QueryResult
	GetComponentByName(name string) *types.Component
}

// MockStorage implements StorageInterface for testing
type MockStorage struct {
	components   map[string][]types.Component
	queryResults map[string]*storage.QueryResult
}

func (m *MockStorage) Close() error { return nil }

func (m *MockStorage) GetComponentsLight(query storage.Query) *storage.QueryResult {
	key := query.Language + ":" + query.Type
	if result, exists := m.queryResults[key]; exists {
		return result
	}
	return &storage.QueryResult{Components: []types.Component{}, Total: 0, HasMore: false}
}

func (m *MockStorage) GetComponentByName(name string) *types.Component {
	for _, components := range m.components {
		for _, comp := range components {
			if comp.Name == name {
				return &comp
			}
		}
	}
	return nil
}

// MockProvider implements providers.OTELCoreProvider for testing
type MockProvider struct {
	language string
	mainSDKs []providers.CorePackage
}

func (m *MockProvider) IsMainSDK(packageName string) bool {
	for _, pkg := range m.mainSDKs {
		if pkg.Name == packageName {
			return true
		}
	}
	return false
}

func (m *MockProvider) GetMainSDKs() []providers.CorePackage {
	return m.mainSDKs
}

// TestKnowledge is a test-specific version of Knowledge that can use mocks
type TestKnowledge struct {
	storage   StorageInterface
	providers map[string]*providers.OTELCoreProvider
	logger    logger.Logger
}

func (kc *TestKnowledge) normalizeLanguage(language string) string {
	switch strings.ToLower(language) {
	case "dotnet", "c#", "csharp":
		return "csharp"
	case "js", "node", "nodejs":
		return "javascript"
	case "py", "python":
		return "python"
	case "go", "golang":
		return "go"
	case "java":
		return "java"
	case "php":
		return "php"
	case "ruby":
		return "ruby"
	default:
		return strings.ToLower(language)
	}
}

func (kc *TestKnowledge) getProvider(language string) *providers.OTELCoreProvider {
	normalizedLanguage := kc.normalizeLanguage(language)
	provider, exists := kc.providers[normalizedLanguage]
	if !exists {
		kc.logger.Logf("Warning: no provider found for language: %s (normalized: %s)\n", language, normalizedLanguage)
		return nil
	}
	return provider
}

func (kc *TestKnowledge) GetCorePackages(language string) ([]string, error) {
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeSDK),
	}

	apiQuery := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeAPI),
	}

	sdkResult := kc.storage.GetComponentsLight(query)
	apiResult := kc.storage.GetComponentsLight(apiQuery)

	var corePackages []string

	provider := kc.getProvider(normalizedLanguage)
	if provider != nil {
		for _, component := range sdkResult.Components {
			if provider.IsMainSDK(component.Name) {
				corePackages = append(corePackages, component.Name)
			}
		}
	} else {
		for _, component := range sdkResult.Components {
			if kc.isMainSDKByPattern(component.Name) {
				corePackages = append(corePackages, component.Name)
			}
		}
	}

	for _, component := range apiResult.Components {
		corePackages = append(corePackages, component.Name)
	}

	return corePackages, nil
}

func (kc *TestKnowledge) IsMainSDK(language, packageName string) bool {
	normalizedLanguage := kc.normalizeLanguage(language)
	provider := kc.getProvider(normalizedLanguage)
	if provider != nil {
		return provider.IsMainSDK(packageName)
	}
	return kc.isMainSDKByPattern(packageName)
}

func (kc *TestKnowledge) GetMainSDKs(language string) ([]providers.CorePackage, error) {
	normalizedLanguage := kc.normalizeLanguage(language)
	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for language: %s", language)
	}
	return provider.GetMainSDKs(), nil
}

func (kc *TestKnowledge) GetPackageType(language, packageName string) (string, error) {
	normalizedLanguage := kc.normalizeLanguage(language)
	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return "", fmt.Errorf("no provider found for language: %s", language)
	}
	return kc.getPackageTypeByPattern(packageName), nil
}

func (kc *TestKnowledge) IsCorePackage(language, packageName string) bool {
	normalizedLanguage := kc.normalizeLanguage(language)
	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return false
	}
	return kc.isMainSDKByPattern(packageName) ||
		strings.Contains(strings.ToLower(packageName), "opentelemetry")
}

func (kc *TestKnowledge) GetInstrumentationPackage(language, instrumentation string) (string, error) {
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeInstrumentation),
		Name:     instrumentation,
	}

	result := kc.storage.GetComponentsLight(query)

	for _, component := range result.Components {
		if isInstrumentationFor(component.Name, instrumentation) {
			return component.Name, nil
		}
	}

	query.Name = ""
	query.Framework = instrumentation
	result = kc.storage.GetComponentsLight(query)

	for _, component := range result.Components {
		if isInstrumentationFor(component.Name, instrumentation) {
			return component.Name, nil
		}
	}

	return "", nil
}

func (kc *TestKnowledge) GetComponentPackage(language, componentType, component string) (string, error) {
	normalizedLanguage := kc.normalizeLanguage(language)

	actualType := mapComponentType(componentType)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     actualType,
		Name:     component,
	}

	result := kc.storage.GetComponentsLight(query)

	for _, comp := range result.Components {
		if comp.Name == component || extractComponentSimpleName(comp.Name) == component {
			return comp.Name, nil
		}
	}

	return "", nil
}

func (kc *TestKnowledge) GetPrerequisites(language string) ([]PrerequisiteRule, error) {
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeInstrumentation),
	}

	result := kc.storage.GetComponentsLight(query)

	var rules []PrerequisiteRule

	for _, component := range result.Components {
		rule := createPrerequisiteRule(component)
		if rule != nil {
			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

func (kc *TestKnowledge) GetComponentsByLanguage(language string, limit, offset int) (*ComponentResult, error) {
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Limit:    limit,
		Offset:   offset,
	}

	result := kc.storage.GetComponentsLight(query)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

func (kc *TestKnowledge) GetComponentsByType(componentType string, limit, offset int) (*ComponentResult, error) {
	query := storage.Query{
		Type:   componentType,
		Limit:  limit,
		Offset: offset,
	}

	result := kc.storage.GetComponentsLight(query)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

func (kc *TestKnowledge) QueryComponents(query ComponentQuery) (*ComponentResult, error) {
	normalizedLanguage := ""
	if query.Language != "" {
		normalizedLanguage = kc.normalizeLanguage(query.Language)
	}

	storageQuery := storage.Query{
		Language:     normalizedLanguage,
		Type:         query.Type,
		Category:     query.Category,
		Status:       query.Status,
		SupportLevel: query.SupportLevel,
		Name:         query.Name,
		Framework:    query.Framework,
		MinDate:      query.MinDate,
		MaxDate:      query.MaxDate,
		Limit:        query.Limit,
		Offset:       query.Offset,
	}

	result := kc.storage.GetComponentsLight(storageQuery)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

func (kc *TestKnowledge) GetComponentByName(name string) (*types.Component, error) {
	return kc.storage.GetComponentByName(name), nil
}

func (kc *TestKnowledge) getPackageTypeByPattern(packageName string) string {
	name := strings.ToLower(packageName)

	if strings.Contains(name, "sdk") {
		return "sdk"
	}
	if strings.Contains(name, "api") {
		return "api"
	}
	if strings.Contains(name, "exporter") {
		return "exporter"
	}
	if strings.Contains(name, "propagator") {
		return "propagator"
	}
	if strings.Contains(name, "instrumentation") {
		return "instrumentation"
	}

	return "component"
}

func (kc *TestKnowledge) isMainSDKByPattern(packageName string) bool {
	if strings.Contains(strings.ToLower(packageName), "instrumentation") {
		return false
	}

	if strings.Contains(strings.ToLower(packageName), "auto-instrumentation") {
		return false
	}

	mainSDKPatterns := []string{
		"sdk-node", "sdk-web", "sdk",
		"otel/sdk",
		"opentelemetry-sdk",
		"OpenTelemetry.Sdk",
		"open-telemetry/sdk",
	}

	for _, pattern := range mainSDKPatterns {
		if strings.Contains(packageName, pattern) {
			return true
		}
	}

	return false
}

// Helper functions needed for testing
// Note: These functions are already defined in knowledge.go, so we don't need to duplicate them here
// The TestKnowledge struct provides access to them through its methods

// createTestKnowledge creates a TestKnowledge instance with mocked dependencies
func createTestKnowledge() *TestKnowledge {
	mockStorage := &MockStorage{
		queryResults: map[string]*storage.QueryResult{
			"csharp:SDK": {
				Components: []types.Component{
					{Name: "OpenTelemetry.Sdk", Type: "SDK", Language: "csharp"},
					{Name: "OpenTelemetry.Extensions.Hosting", Type: "SDK", Language: "csharp"},
				},
				Total: 2,
			},
			"csharp:API": {
				Components: []types.Component{
					{Name: "OpenTelemetry.Api", Type: "API", Language: "csharp"},
					{Name: "OpenTelemetry", Type: "API", Language: "csharp"},
				},
				Total: 2,
			},
			"javascript:SDK": {
				Components: []types.Component{
					{Name: "@opentelemetry/sdk-node", Type: "SDK", Language: "javascript"},
				},
				Total: 1,
			},
		},
	}

	mockProviders := map[string]*providers.OTELCoreProvider{
		"csharp":     &providers.OTELCoreProvider{},
		"javascript": &providers.OTELCoreProvider{},
	}

	return &TestKnowledge{
		storage:   mockStorage,
		providers: mockProviders,
		logger:    &logger.StdoutLogger{},
	}
}

func TestNormalizeLanguage(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"dotnet", "csharp"},
		{"c#", "csharp"},
		{"csharp", "csharp"},
		{"C#", "csharp"},
		{"DOTNET", "csharp"},
		{"javascript", "javascript"},
		{"js", "javascript"},
		{"node", "javascript"},
		{"nodejs", "javascript"},
		{"python", "python"},
		{"py", "python"},
		{"go", "go"},
		{"golang", "go"},
		{"java", "java"},
		{"php", "php"},
		{"ruby", "ruby"},
		{"unknown", "unknown"},
	}

	kc := &Knowledge{}

	for _, test := range tests {
		result := kc.normalizeLanguage(test.input)
		if result != test.expected {
			t.Errorf("normalizeLanguage(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestGetCorePackages(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	packages, err := kc.GetCorePackages("dotnet")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	if len(packages) == 0 {
		t.Error("Expected to find core packages for dotnet")
	}

	// Test with "csharp" (should work directly)
	packages, err = kc.GetCorePackages("csharp")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	if len(packages) == 0 {
		t.Error("Expected to find core packages for csharp")
	}

	// Test with "c#" (should normalize to "csharp")
	packages, err = kc.GetCorePackages("c#")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	if len(packages) == 0 {
		t.Error("Expected to find core packages for c#")
	}

	// Test with unknown language
	packages, err = kc.GetCorePackages("unknown")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	// Should return empty list for unknown language
	if len(packages) != 0 {
		t.Errorf("Expected no packages for unknown language, got %d", len(packages))
	}
}

func TestGetCorePackagesWithJavaScript(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "js" (should normalize to "javascript")
	packages, err := kc.GetCorePackages("js")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	if len(packages) == 0 {
		t.Error("Expected to find core packages for js")
	}

	// Test with "node" (should normalize to "javascript")
	packages, err = kc.GetCorePackages("node")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	if len(packages) == 0 {
		t.Error("Expected to find core packages for node")
	}
}

func TestGetCorePackagesWithPython(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "py" (should normalize to "python")
	packages, err := kc.GetCorePackages("py")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	// Should return empty list since we don't have python data in mock
	if len(packages) != 0 {
		t.Errorf("Expected no packages for py, got %d", len(packages))
	}
}

func TestGetCorePackagesWithGo(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "golang" (should normalize to "go")
	packages, err := kc.GetCorePackages("golang")
	if err != nil {
		t.Fatalf("GetCorePackages failed: %v", err)
	}
	// Should return empty list since we don't have go data in mock
	if len(packages) != 0 {
		t.Errorf("Expected no packages for golang, got %d", len(packages))
	}
}

func TestIsMainSDK(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	result := kc.IsMainSDK("dotnet", "OpenTelemetry.Sdk")
	if !result {
		t.Error("Expected IsMainSDK to return true for dotnet + OpenTelemetry.Sdk")
	}

	// Test with "csharp" (should work directly)
	result = kc.IsMainSDK("csharp", "OpenTelemetry.Sdk")
	if !result {
		t.Error("Expected IsMainSDK to return true for csharp + OpenTelemetry.Sdk")
	}

	// Test with "c#" (should normalize to "csharp")
	result = kc.IsMainSDK("c#", "OpenTelemetry.Sdk")
	if !result {
		t.Error("Expected IsMainSDK to return true for c# + OpenTelemetry.Sdk")
	}
}

func TestGetMainSDKs(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	sdks, err := kc.GetMainSDKs("dotnet")
	if err != nil {
		t.Fatalf("GetMainSDKs failed: %v", err)
	}
	if len(sdks) == 0 {
		t.Error("Expected to find main SDKs for dotnet")
	}

	// Test with "csharp" (should work directly)
	sdks, err = kc.GetMainSDKs("csharp")
	if err != nil {
		t.Fatalf("GetMainSDKs failed: %v", err)
	}
	if len(sdks) == 0 {
		t.Error("Expected to find main SDKs for csharp")
	}
}

func TestGetPackageType(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	pkgType, err := kc.GetPackageType("dotnet", "OpenTelemetry.Sdk")
	if err != nil {
		t.Fatalf("GetPackageType failed: %v", err)
	}
	if pkgType == "" {
		t.Error("Expected to get package type for dotnet")
	}

	// Test with "csharp" (should work directly)
	pkgType, err = kc.GetPackageType("csharp", "OpenTelemetry.Sdk")
	if err != nil {
		t.Fatalf("GetPackageType failed: %v", err)
	}
	if pkgType == "" {
		t.Error("Expected to get package type for csharp")
	}
}

func TestIsCorePackage(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	result := kc.IsCorePackage("dotnet", "OpenTelemetry.Sdk")
	if !result {
		t.Error("Expected IsCorePackage to return true for dotnet + OpenTelemetry.Sdk")
	}

	// Test with "csharp" (should work directly)
	result = kc.IsCorePackage("csharp", "OpenTelemetry.Sdk")
	if !result {
		t.Error("Expected IsCorePackage to return true for csharp + OpenTelemetry.Sdk")
	}
}

func TestGetInstrumentationPackage(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	pkg, err := kc.GetInstrumentationPackage("dotnet", "test-instrumentation")
	if err != nil {
		t.Fatalf("GetInstrumentationPackage failed: %v", err)
	}
	// Should return empty string since we don't have instrumentation data in mock
	if pkg != "" {
		t.Errorf("Expected empty string for unknown instrumentation, got %s", pkg)
	}

	// Test with "csharp" (should work directly)
	pkg, err = kc.GetInstrumentationPackage("csharp", "test-instrumentation")
	if err != nil {
		t.Fatalf("GetInstrumentationPackage failed: %v", err)
	}
	if pkg != "" {
		t.Errorf("Expected empty string for unknown instrumentation, got %s", pkg)
	}
}

func TestGetComponentPackage(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	pkg, err := kc.GetComponentPackage("dotnet", "SDK", "OpenTelemetry.Sdk")
	if err != nil {
		t.Fatalf("GetComponentPackage failed: %v", err)
	}
	// Should return empty string since we don't have exact match in mock
	if pkg != "" {
		t.Errorf("Expected empty string for unknown component, got %s", pkg)
	}

	// Test with "csharp" (should work directly)
	pkg, err = kc.GetComponentPackage("csharp", "SDK", "OpenTelemetry.Sdk")
	if err != nil {
		t.Fatalf("GetComponentPackage failed: %v", err)
	}
	if pkg != "" {
		t.Errorf("Expected empty string for unknown component, got %s", pkg)
	}
}

func TestGetPrerequisites(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	rules, err := kc.GetPrerequisites("dotnet")
	if err != nil {
		t.Fatalf("GetPrerequisites failed: %v", err)
	}
	// Should return empty list since we don't have prerequisite data in mock
	if len(rules) != 0 {
		t.Errorf("Expected no prerequisite rules, got %d", len(rules))
	}

	// Test with "csharp" (should work directly)
	rules, err = kc.GetPrerequisites("csharp")
	if err != nil {
		t.Fatalf("GetPrerequisites failed: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("Expected no prerequisite rules, got %d", len(rules))
	}
}

func TestGetComponentsByLanguage(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	result, err := kc.GetComponentsByLanguage("dotnet", 10, 0)
	if err != nil {
		t.Fatalf("GetComponentsByLanguage failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for dotnet")
	}

	// Test with "csharp" (should work directly)
	result, err = kc.GetComponentsByLanguage("csharp", 10, 0)
	if err != nil {
		t.Fatalf("GetComponentsByLanguage failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for csharp")
	}
}

func TestGetComponentsByType(t *testing.T) {
	kc := createTestKnowledge()

	// Test with valid component type
	result, err := kc.GetComponentsByType("SDK", 10, 0)
	if err != nil {
		t.Fatalf("GetComponentsByType failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for SDK type")
	}
}

func TestQueryComponents(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" language
	query := ComponentQuery{
		Language: "dotnet",
		Type:     "SDK",
		Limit:    10,
		Offset:   0,
	}
	result, err := kc.QueryComponents(query)
	if err != nil {
		t.Fatalf("QueryComponents failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for dotnet query")
	}

	// Test with "csharp" language
	query.Language = "csharp"
	result, err = kc.QueryComponents(query)
	if err != nil {
		t.Fatalf("QueryComponents failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for csharp query")
	}

	// Test with empty language (should not normalize)
	query.Language = ""
	result, err = kc.QueryComponents(query)
	if err != nil {
		t.Fatalf("QueryComponents failed: %v", err)
	}
	if result == nil {
		t.Error("Expected non-nil result for empty language query")
	}
}

func TestGetComponentByName(t *testing.T) {
	kc := createTestKnowledge()

	// Test with existing component name
	component, err := kc.GetComponentByName("OpenTelemetry.Sdk")
	if err != nil {
		t.Fatalf("GetComponentByName failed: %v", err)
	}
	if component == nil {
		t.Error("Expected to find component OpenTelemetry.Sdk")
	}

	// Test with non-existing component name
	component, err = kc.GetComponentByName("NonExistentComponent")
	if err != nil {
		t.Fatalf("GetComponentByName failed: %v", err)
	}
	if component != nil {
		t.Error("Expected nil for non-existent component")
	}
}

func TestGetProvider(t *testing.T) {
	kc := createTestKnowledge()

	// Test with "dotnet" (should normalize to "csharp")
	provider := kc.getProvider("dotnet")
	if provider == nil {
		t.Error("Expected to find provider for dotnet")
	}

	// Test with "csharp" (should work directly)
	provider = kc.getProvider("csharp")
	if provider == nil {
		t.Error("Expected to find provider for csharp")
	}

	// Test with unknown language
	provider = kc.getProvider("unknown")
	if provider != nil {
		t.Error("Expected nil provider for unknown language")
	}
}

func TestGetPackageTypeByPattern(t *testing.T) {
	kc := createTestKnowledge()

	tests := []struct {
		input    string
		expected string
	}{
		{"opentelemetry-sdk", "sdk"},
		{"opentelemetry-api", "api"},
		{"opentelemetry-exporter-otlp", "exporter"},
		{"opentelemetry-propagator-b3", "propagator"},
		{"opentelemetry-instrumentation-http", "instrumentation"},
		{"unknown-package", "component"},
	}

	for _, test := range tests {
		result := kc.getPackageTypeByPattern(test.input)
		if result != test.expected {
			t.Errorf("getPackageTypeByPattern(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

func TestIsMainSDKByPattern(t *testing.T) {
	kc := createTestKnowledge()

	tests := []struct {
		input    string
		expected bool
	}{
		{"opentelemetry-sdk", true},
		{"@opentelemetry/sdk-node", true},
		{"go.opentelemetry.io/otel/sdk", true},
		{"OpenTelemetry.Sdk", true},
		{"opentelemetry-instrumentation-express", false},
		{"opentelemetry-auto-instrumentations-web", false},
		{"unknown-package", false},
	}

	for _, test := range tests {
		result := kc.isMainSDKByPattern(test.input)
		if result != test.expected {
			t.Errorf("isMainSDKByPattern(%q) = %v, expected %v", test.input, result, test.expected)
		}
	}
}
