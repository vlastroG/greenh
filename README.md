# greenh

A hobby automatic plant waterer. For each pot it tells you whether the plant needs water and how much.

Hardware is simulated for now. A Go backend stores sensor readings in SQLite and asks an LLM (any OpenAI-compatible API) or built-in rules for advice. A web dashboard shows the results.

**Status:** early prototype, work in progress.

## Quick start

1. `cp .env.example .env` and set `ADVISOR_API_KEY` (without it the rules advisor is used).
2. `docker compose up --build`
3. Open http://localhost:8080

## Docs

- [Design spec](docs/plant-waterer-design.md)
- [Implementation plans](docs/superpowers/plans/)

## License

[MIT](LICENSE)
