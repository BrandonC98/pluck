# Instructions for AI agents

## Rules
- Use go 1.27.0 as the programming language
- Make code clear and simple where possible
- only introduce new tools with confirmation
- Use homebrew files to store tooling where possible
- Use makefile to store common commands
- use viper and cobra frameworks where needed

## Testing
- Use the testify testing framework to create unit tests in a test driven development style
- Tests should be run as part of validation that code changes are working as expected
- Tests should be run as part of a  CI workflow using github actions

## boundaries 
- Don't hardcode any secrets
