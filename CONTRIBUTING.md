# Contributing to GoEvent

Thank you for your interest in contributing to GoEvent! We welcome contributions from the community.

## How to Contribute

### Reporting Issues

- Check if the issue already exists in the [issue tracker](https://github.com/openframebox/goevent/issues)
- Use a clear and descriptive title
- Provide a minimal reproducible example
- Include Go version, OS, and any relevant environment details

### Submitting Pull Requests

1. **Fork the repository** and create your branch from `main`
   ```bash
   git checkout -b feature/my-new-feature
   ```

2. **Make your changes**
   - Write clear, idiomatic Go code
   - Add tests for new functionality
   - Update documentation as needed
   - Follow the existing code style

3. **Test your changes**
   ```bash
   go test ./...
   go vet ./...
   ```

4. **Commit your changes**
   - Use clear, descriptive commit messages
   - Reference related issues in commit messages

5. **Push to your fork** and submit a pull request
   ```bash
   git push origin feature/my-new-feature
   ```

6. **Wait for review** - A maintainer will review your PR and may request changes

## Code Style Guidelines

- Follow standard Go conventions and idioms
- Run `go fmt` before committing
- Use meaningful variable and function names
- Add godoc comments for all exported types, functions, and methods
- Keep functions focused and reasonably sized

## Testing Guidelines

- Write table-driven tests where appropriate
- Test both success and error cases
- Aim for good test coverage of critical paths
- Use descriptive test names that explain what is being tested

Example:
```go
func TestDispatch_WithAsyncListener_WaitsCorrectly(t *testing.T) {
    // Test implementation
}
```

## Documentation

- Update README.md if adding new features
- Add godoc comments for exported symbols
- Include usage examples for new functionality

## Questions?

Feel free to open an issue with the `question` label if you need help or clarification.

## License

By contributing to GoEvent, you agree that your contributions will be licensed under the MIT License.
