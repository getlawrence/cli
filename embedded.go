package main

import "embed"

//go:embed data/knowledge.db
var EmbeddedKnowledgeDB embed.FS
