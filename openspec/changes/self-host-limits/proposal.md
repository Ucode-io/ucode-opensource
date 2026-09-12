# Limits of a self-hosted installation

> **Parked.** The idea is agreed in principle; the actual list of limits is the
> product owner's to write. Nothing here is implemented. Do not start on it
> until that list exists.

## Why

ucode is open-core. Some capability has to stay on the hosted side, or there is
no hosted product — but where the line runs has never been written down, and
right now it is not enforced anywhere on purpose.

One limit already exists by accident, which is what raised the question.
`services/company/grpc/service/company.go:43`:

```go
if companies.Count >= 1 && s.cfg.Environment != config.PRODUCTION {
    return error "only one company allowed"
}
```

Read it twice: the restriction applies **everywhere except production**, and
`ENVIRONMENT` defaults to `production`. So a stock installation has no limit at
all. It fired during local testing only because the compose file sets
`ENVIRONMENT=debug`.

That is a developer's guard against creating stray companies on a laptop, not a
product rule. A product rule would be the other way round.

## What

Turn the boundary into something explicit, documented and independent of the
debug flag.

The shape, once the list exists:

- Each limit is its own setting with a name that says what it is, not a side
  effect of a mode intended for something else.
- Defaults ship in `deploy/.env.example`, so an operator can see the boundary
  without reading Go.
- Exceeding one returns a message a human understands — "this installation is
  licensed for one company" — not `codes.Internal`.
- The README states the boundary plainly. A limit nobody documented reads as a
  bug and gets reported as one.

### The candidate that prompted this

One company per installation. Defensible: a self-hosted ucode serves one
organisation with as many projects inside it as it likes; several organisations
in one installation is what the hosted product is for. It costs a single-team
user nothing.

## What is needed before starting

The list of limits, from the product owner. Until then this stays parked: the
mechanism is easy, deciding where the line runs is not, and guessing at it would
put a boundary in the code that nobody agreed to.
