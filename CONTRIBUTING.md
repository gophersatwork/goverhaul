# Contributing to Goverhaul

Thank you for considering contributing to Goverhaul! This document outlines the process for contributing to the project and how to get started with development.

## Code of Conduct

By participating in this project, you are expected to uphold our [Code of Conduct](CODE_OF_CONDUCT.md).

## How Can I Contribute?

### Reporting Bugs

This section guides you through submitting a bug report for Goverhaul. Following these guidelines helps maintainers understand your report, reproduce the behavior, and find related reports.

Before creating bug reports, please check [the issue list](https://github.com/gophersatwork/goverhaul/issues) as you might find out that you don't need to create one. When you are creating a bug report, please include as many details as possible:

* **Use a clear and descriptive title** for the issue to identify the problem.
* **Describe the exact steps which reproduce the problem** in as many details as possible.
* **Provide specific examples to demonstrate the steps**. Include links to files or GitHub projects, or copy/pasteable snippets, which you use in those examples.
* **Describe the behavior you observed after following the steps** and point out what exactly is the problem with that behavior.
* **Explain which behavior you expected to see instead and why.**
* **Include screenshots and animated GIFs** which show you following the described steps and clearly demonstrate the problem.
* **If the problem wasn't triggered by a specific action**, describe what you were doing before the problem happened.

### Suggesting Enhancements

This section guides you through submitting an enhancement suggestion for Goverhaul, including completely new features and minor improvements to existing functionality.

* **Use a clear and descriptive title** for the issue to identify the suggestion.
* **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
* **Provide specific examples to demonstrate the steps**. Include copy/pasteable snippets which you use in those examples.
* **Describe the current behavior** and **explain which behavior you expected to see instead** and why.
* **Explain why this enhancement would be useful** to most Goverhaul users.

### Pull Requests

* Fill in the required template
* Do not include issue numbers in the PR title
* Include screenshots and animated GIFs in your pull request whenever possible
* Follow the [Go style guide](#go-style-guide)
* Include tests for new features or bug fixes
* Document new code based on the [Documentation Styleguide](#documentation-styleguide)
* End all files with a newline

## Development Setup

### Prerequisites

* Go 1.18 or higher
* Git

### Getting Started

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/yourusername/goverhaul.git
   cd goverhaul
   ```
3. Create a branch for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. Make your changes
5. Run tests to ensure everything works:
   ```bash
   go test ./...
   ```
6. Commit your changes:
   ```bash
   git commit -m "Add your meaningful commit message here"
   ```
7. Push to your fork:
   ```bash
   git push origin feature/your-feature-name
   ```
8. Create a Pull Request from your fork to the main repository

## Go Style Guide

* Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
* Format your code with `gofmt` or `go fmt`
* Use meaningful variable and function names
* Write comments for exported functions, types, and constants
* Keep functions small and focused on a single responsibility
* Use proper error handling

## Documentation Styleguide

* Use [Markdown](https://guides.github.com/features/mastering-markdown/) for documentation
* Reference functions, classes, and modules in backticks: \`func Example()\`
* Use code blocks for examples:
  ```go
  func Example() {
      // Your code here
  }
  ```

## Release Process

The release process is documented in [RELEASE.md](RELEASE.md).

## Additional Notes

### Issue and Pull Request Labels

This section lists the labels we use to help us track and manage issues and pull requests.

* `bug` - Issues that are bugs
* `documentation` - Issues or PRs related to documentation
* `enhancement` - Issues that are feature requests or PRs that implement new features
* `good first issue` - Good for newcomers
* `help wanted` - Extra attention is needed
* `question` - Further information is requested

## Thank You!

Your contributions to open source, large or small, make projects like this possible. Thank you for taking the time to contribute.