# Security

## Reporting a vulnerability

Please do not open a public issue for a security problem.

Report it through GitHub's private vulnerability reporting on this repository
(Security → Report a vulnerability). Include what you found, how to reproduce
it, and what an attacker could do with it.

You will get an acknowledgement within a few working days.

## Supported versions

This is pre-release software; only the current `main` is supported. Once there
are tagged releases this section will say which ones receive fixes.

## Running ucode safely

The local stack is built for development and ships accordingly:

- It binds to `127.0.0.1` only.
- It creates an administrator with a known password.
- The generated `SECRET_KEY` in `~/.ucode/.env` signs every token.

None of that is safe on a reachable network. Before exposing an installation to
anything beyond your own machine, change the administrator password, keep the
signing key secret, and put the services behind TLS.
