package languages

type LanguageRegistry struct {
	plugins map[string]LanguagePlugin
}

// DefaultRegistry is the global language registry used by the application.
var DefaultRegistry = NewLanguageRegistry()

func NewLanguageRegistry() *LanguageRegistry {
	return &LanguageRegistry{
		plugins: make(map[string]LanguagePlugin),
	}
}

func (r *LanguageRegistry) Register(plugin LanguagePlugin) {
	r.plugins[plugin.ID()] = plugin
}

func (r *LanguageRegistry) Get(id string) (LanguagePlugin, bool) {
	p, ok := r.plugins[id]
	return p, ok
}

func (r *LanguageRegistry) All() map[string]LanguagePlugin {
	// Return a shallow copy to avoid external mutation
	out := make(map[string]LanguagePlugin, len(r.plugins))
	for k, v := range r.plugins {
		out[k] = v
	}
	return out
}
