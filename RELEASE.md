# Release Process

This document outlines the process for creating and publishing releases of Goverhaul.

## Version Numbering

Goverhaul follows [Semantic Versioning](https://semver.org/) (SemVer):

- **MAJOR** version for incompatible API changes
- **MINOR** version for backward-compatible functionality additions
- **PATCH** version for backward-compatible bug fixes

## Release Checklist

Before creating a new release, ensure the following steps are completed:

1. **Update Documentation**
   - Ensure README.md is up to date
   - Update any relevant documentation files

2. **Run Tests**
   - Ensure all checks are passing: `make check`

3. **Version Update**
   - Update version number in relevant files (if applicable)

## Creating a Release

1. **Create a Release Branch**
   ```bash
   git checkout -b release/vX.Y.Z
   ```

2. **Commit Changes**
   ```bash
   git add .
   git commit -m "Prepare release vX.Y.Z"
   ```

3. **Create a Pull Request**
   - Create a PR from the release branch to main
   - Get the PR reviewed and approved
   - Merge the PR

4. **Tag the Release**
   ```bash
   git checkout main
   git pull
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ```

5. **Create GitHub Release**
   - Go to the [Releases page](https://github.com/gophersatwork/goverhaul/releases)
   - Click "Draft a new release"
   - Select the tag you just pushed
   - Title the release "vX.Y.Z"
   - Copy the relevant section from CHANGELOG.md into the description
   - Attach any relevant binaries or artifacts
   - Publish the release

## Post-Release

1. **Announce the Release**
   - Announce the new release in relevant channels

2. **Prepare for Next Development Cycle**
   - Create a new "Unreleased" section in CHANGELOG.md
   - Update version numbers in development files if necessary

## Hotfix Releases

For urgent fixes that can't wait for the next regular release:

1. Create a branch from the release tag: `git checkout -b hotfix/vX.Y.Z+1 vX.Y.Z`
2. Make the necessary fixes
3. Follow the regular release process, but increment only the PATCH version

## Release Automation

Some parts of the release process are automated through GitHub Actions:

- Tests run automatically on PRs and pushes to main
- The release workflow is triggered when a new tag is pushed
- Binaries are automatically built and attached to the GitHub Release

## Supported Versions

Generally, only the latest version is actively supported. For details on which versions receive security updates, see [SECURITY.md](SECURITY.md).