# Contributing

Issues are welcome: a bug, a machine the installer fails on, a wrong or missing entry in the catalog, or a
security design you think is weak (for an actual vulnerability, see SECURITY.md first).

Pull requests are welcome too. Small ones are merged fastest. Before sending one, run what CI runs:

```
cd node/web && npm ci && npx tsc --noEmit && node build.mjs
cd node && go vet ./... && go test -race ./...
cd site && npm ci && npx eslint . && npx next build
```

Contributions are accepted under the Apache License 2.0, the licence of the project. Releases are built and
signed by the maintainer; nothing in CI holds the signing key, and a pull request cannot deploy the website.
