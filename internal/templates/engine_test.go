package templates

import "testing"

func TestTemplateEngine_LoadsTemplates(t *testing.T) {
	eng, err := NewTemplateEngine()
	if err != nil {
		t.Fatalf("NewTemplateEngine error: %v", err)
	}
	if len(eng.GetAvailableTemplates()) == 0 {
		t.Fatalf("expected some embedded templates to be available")
	}
}

func TestTemplateEngine_GenerateAgentPrompt_Missing(t *testing.T) {
	eng, err := NewTemplateEngine()
	if err != nil {
		t.Fatalf("NewTemplateEngine error: %v", err)
	}
	// Agent prompt template exists; verify it returns some text without error
	_, err = eng.GenerateAgentPrompt(AgentPromptData{Language: "go", Instructions: "do x"})
	if err != nil {
		t.Fatalf("GenerateAgentPrompt unexpected error: %v", err)
	}
}
