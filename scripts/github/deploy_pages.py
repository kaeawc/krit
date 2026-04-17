#!/usr/bin/env python3
"""Build, serve, or deploy krit documentation with MkDocs."""

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent


def build_docs() -> None:
    """Build the documentation site with mkdocs build --strict."""
    subprocess.run(
        ["mkdocs", "build", "--strict"],
        cwd=ROOT,
        check=True,
    )
    print("Documentation built successfully in site/")


def serve_docs() -> None:
    """Serve the documentation locally for preview."""
    print("Starting local preview at http://127.0.0.1:8000")
    subprocess.run(
        ["mkdocs", "serve"],
        cwd=ROOT,
        check=True,
    )


def deploy_docs() -> None:
    """Deploy the documentation to GitHub Pages."""
    subprocess.run(
        ["mkdocs", "gh-deploy", "--clean"],
        cwd=ROOT,
        check=True,
    )
    print("Documentation deployed to GitHub Pages")


def main() -> None:
    commands = {
        "build": build_docs,
        "serve": serve_docs,
        "deploy": deploy_docs,
    }

    if len(sys.argv) < 2 or sys.argv[1] not in commands:
        print(f"Usage: {sys.argv[0]} [{' | '.join(commands)}]")
        sys.exit(1)

    commands[sys.argv[1]]()


if __name__ == "__main__":
    main()
