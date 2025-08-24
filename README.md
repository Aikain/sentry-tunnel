# sentry-tunnel

* [Sentry for React - `tunnel` option](https://docs.sentry.io/platforms/javascript/guides/react/configuration/options/#tunnel)
* [Using the `tunnel` Option](https://docs.sentry.io/platforms/javascript/troubleshooting/#using-the-tunnel-option)

## Envs

| Name               | Description                                       | Default       | Example   |
|--------------------|---------------------------------------------------|---------------|-----------|
| SENTRY_HOST        | Sentry address to which requests can be forwarded | _(allow all)_ | sentry.io |
| SENTRY_PROJECT_IDS | IDs of allowed projects separated by commas       | _(allow all)_ | 3,5,15    |
| TUNNEL_PATH        | The path where requests come in                   | /tunnel       |           |
| PORT               | The port the application listens on               | 8090          |           |
