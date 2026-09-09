# Cutting the open-source core

> Recorded after the fact. This work happened before OpenSpec was introduced,
> so there is no matching spec or task list — but the decisions are worth
> keeping, because the code they removed is not in the history of this
> repository to look at.

## Why

ucode is an open-core product. The platform a developer can self-host is the
data engine, the permission model, the entry point and the visual builder.
The AI that generates applications, the payment system and the third-party
integrations are what the hosted product sells, and they stay closed.

Publishing the whole codebase would have given that away, and would also have
shipped something nobody could run: two thirds of the gateway needed services,
credentials or an outside CI system that a laptop does not have.

## What was removed

Roughly 98,000 lines across the gateway and company-service.

| | Why |
|---|---|
| AI module: prompts, agents, tool use, MCP, Anthropic/OpenAI/Gemini clients | The prompts are the product. 8,100 lines of prompt engineering for web apps, admin panels, landing pages and mobile — years of accumulated work, copyable in five minutes |
| Billing: transactions, subscriptions, fares, payment providers, seat charges, usage metering | Commercial. Also reached further than expected: API-call counting middleware sat on every route group, and quota checks fired from item and file CRUD |
| Every third-party integration: CRM assistant, Meta and Facebook Lead Ads, Telegram, Meta Ads, Google Drive and Calendar, GitHub, GitLab, Unsplash, n8n, Grafana, Yandex Metrica, KP generator, Capacitor | Built for named customers. Not general-purpose platform features |
| Microfrontends and serverless functions | Depend on Knative and GitLab CI, which self-hosting cannot supply |

Two services are therefore published in truncated form: the gateway without its
AI module, company-service without payments. The rest are whole.

## What the cut also removed, incidentally

Things that had no business being in a public repository and left with the code
around them:

- A live OpenAI API key, shipped as a config default
- A Redis password and two service secrets, likewise
- A JWT signing key compiled into auth-service, so every installation would
  have signed tokens with the same value
- Production database credentials in four test files, one of which made
  `go test ./...` hang for ten minutes against a live server
- A customer's tenant seeded by an auth migration: company name, three project
  names, an admin login, a phone number and an employee's email address
- A hardcoded project id in the gateway routing one named customer to a
  separate cluster

## Method

Delete, then let the compiler find every loose end. Go makes this reliable:
each removal surfaces its callers as build errors, and 158 dead routes were
removed by feeding the compiler's own line numbers back into the file.

Billing went differently. Its RPCs were declared in proto, so the service was
obliged to implement them; deleting Go code alone would not compile. The proto
contract was cut first and regenerated — after a dry run confirmed that
regenerating with no changes reproduced the committed output byte for byte.

## What remains

- Plan-limit checks in auth-service are stubs that always pass, rather than
  being removed from their eight call sites
- `user_seat_billing.go` and a `BillingServiceClient` still sit in
  auth-service's gRPC client interface
- The `Billing` field on the permission model in object-builder is inert data
