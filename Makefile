.PHONY: setup lint clean

setup:
	@echo "Install experiment dependencies:"
	@echo "  pip install -r experiments/auth/phase2-poc/cloud-function/requirements.txt"
	@echo "  pip install -r experiments/ho-platform-none/install-ho-platform-none/requirements.txt"

lint:
	python -m ruff check . 2>/dev/null || echo "No linter installed"

clean:
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null
	find . -type f -name '*.pyc' -delete 2>/dev/null
