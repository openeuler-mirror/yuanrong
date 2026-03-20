# Example Layout

The `example/` directory is organized by scenario instead of keeping all
artifacts at the top level.

- `aio/`: all-in-one example environment and legacy reference assets.
- `casdoor/`: Casdoor-based authentication setup and bootstrap scripts.
- `keycloak/`: Keycloak-based authentication setup and bootstrap scripts.
- `others/`: small standalone sample files kept for reference.
- `traefik/`: reverse proxy configuration used by auth examples.
- `webterminal/`: webterminal demo runtime and launch scripts.

Top-level helper scripts kept here:

- `restart.sh`: reinstall package artifacts and restart Yuanrong.
- `restart_local.sh`: restart without reinstalling wheel artifacts.
