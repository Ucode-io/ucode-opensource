# Tasks

## 1. Make the boundary a rule instead of an accident

- [x] 1.1 Both guards existed already and both were backwards. The company one
      and the project one each read `cfg.Environment != PRODUCTION`, so they
      applied everywhere *except* production — and `ENVIRONMENT` defaults to
      `production`, which meant a stock installation had no limit at all. They
      fired on the local stack only because its compose file happens to set
      `ENVIRONMENT=debug`
- [x] 1.2 `MAX_COMPANIES` and `MAX_PROJECTS`, each its own setting, read in
      company-service's config, defaulting to 1. Zero means no limit, so an
      operator who wants more says so deliberately rather than by changing an
      unrelated mode
- [x] 1.3 One `selfHostLimit` type carries the decision and the message, so
      the two guards cannot drift apart
- [x] 1.4 `codes.FailedPrecondition`, not `codes.Internal`. Nothing went
      wrong; the request asked for something this installation does not do
- [x] 1.5 A message a person can act on: "this installation is set up for 1
      company (MAX_COMPANIES=1). Raise MAX_COMPANIES if you need more."
- [x] 1.6 While rewriting the company guard: it counted companies with an
      error it never checked, so a failed count fell through to creating the
      company anyway

## 2. Everything that reads the old message

- [x] 2.1 The CLI treats "cannot create a second company" as "already set up"
      — that is what a re-run with a lost marker file looks like. It matched
      the old sentence verbatim. It now matches `MAX_COMPANIES`, the part of
      the message that will survive rewording, and still recognises the old
      phrasing so a CLI from this version can talk to images published before
      it
- [x] 2.2 `deploy/.env.example` and the compose file carry both settings, with
      `:-1` defaults in compose so an older `.env` still works
- [x] 2.3 README states the boundary in its own section, including that
      everything inside one project stays unlimited

## 3. Proof

- [x] 3.1 Unit tests on the limit: allows up to the maximum, zero means
      unlimited, the refusal carries the right code and a readable message,
      and the plural is right above one. Mutation-checked — off-by-one,
      dropping the zero case, and swapping the status code each turn a test
      red
- [x] 3.2 Unit test on the CLI matcher, including that a still-starting
      service and a genuine failure are *not* read as "already set up".
      Mutation-checked
- [ ] 3.3 Confirm against a running stack: create a second project through the
      admin panel and read the refusal. Needs the stack up, so it goes with
      the acceptance walk
