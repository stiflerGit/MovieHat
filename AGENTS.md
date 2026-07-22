# AGENTS.md

Instructions for AI coding agents working in this repository.

## Code modification policy

Do **not** modify business code unless the user explicitly includes the phrase:

```text
ALLOW CODE CHANGES
```

Do **not** modify test files unless the user explicitly includes either phrase:

```text
ALLOW TEST CHANGES
```

or:

```text
ALLOW CODE CHANGES
```

`ALLOW TEST CHANGES` permits changes to test files only. It does not permit changes to business code.

Business code includes, but is not limited to:

- application/domain logic
- handlers and services
- persistence/storage code
- migrations
- generated API code
- protobuf definitions
- tests that change expectations around business behavior
- configuration or startup code that affects runtime behavior

If the user asks for a review, brainstorming, diagnosis, test fixing, refactoring, or implementation but does **not** include the required allow phrase, you must not edit those files. Instead, explain what you would change and ask for explicit permission.

## Allowed without `ALLOW CODE CHANGES`

You may perform read-only actions, such as:

- inspect files
- run tests or linters
- run search commands
- provide code review feedback
- propose patches in prose

You may create or update documentation-only instruction files when explicitly requested, such as this `AGENTS.md`.

## When in doubt

Ask before editing.
