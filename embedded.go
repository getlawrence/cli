package main

import "embed"

//go:embed knowledge.db
var EmbeddedKnowledgeDB embed.FS
