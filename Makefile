.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

.PHONY: walkthrough
walkthrough: ## Launch an interactive AI-guided codebase exploration
	@command -v claude >/dev/null 2>&1 || \
	  { echo "Error: claude CLI not found. Please install Claude Code."; exit 1; }
	claude --agent walkthrough --permission-mode auto \
	  --strict-mcp-config \
	  --mcp-config .claude/walkthrough/mcp.json \
	  --settings .claude/walkthrough/settings.json
