# Remove what belongs to the hosted product

## Why

The cut that produced this repository removed the billing service code — the
proto copies went, `BillingServiceClient` went, the limit helpers became
no-ops, and the compiled binaries prove it: `BillingService` appears zero times
in any of the four.

The **schema and the seed data** were never touched. Migrations are not Go
code, so nothing the compiler said pointed at them, and they went out with
everything else. They ship inside the `migrate` image, which was made public on
ghcr on 2026-09-12.

What that image carried:

| | |
|---|---|
| ucode price list | Small $300, Medium $600, Private $100 |
| Ugen price list | Free $0, Basic $300, Pro $600 |
| Per-tier quotas | 14 metered dimensions each — builders, projects, seats, AI credits per day and per month, requests per month and per second, functions, microfrontends, database size, asset size, items, tables, API keys |
| Term discounts | 13% for six months, 24% annual |
| Billing tables | `fare`, `fare_item`, `fare_item_price`, `subscription`, `transaction`, `billing_usage`, `company_token_balance`, `project_user_seat_billing_period`, `ugen_billing_period` |

A second surface is still in: Ugen. Its TypeScript lives in another repository
and is not here, but the backend that serves it is — `/v3/ugen/*` in auth,
`/v1/ugen/*` in the gateway, the whole `ugen_template` catalogue in company,
and `ugen_template.proto`. 33 hand-written files, 526 lines, 32 generated.

## What

Two phases, because they carry different risk.

**Phase 1 — the pricing data.** Remove every seeded fare, fare item, fare item
price and billing period. Keep the tables. Nothing in the open-source build
reads them: the two live references in `project.go` are a correlated scalar
subquery and a `LEFT JOIN`, both of which return NULL against an empty table
rather than dropping the row.

**Phase 2 — Ugen.** Delete the routes, handlers, storage and proto, and
regenerate. Larger, and it touches the contract surface, so it goes second.

## What this is not

Not a decision about what the hosted product charges, and not the
[self-host limits](../self-host-limits/proposal.md) question. This only removes
things the open-source build cannot use.

## Note on already-published images

The images published before this change remain pullable and still contain the
price lists. Fixing the repository does not recall them. Deleting those package
versions from ghcr is a separate decision for the owner.
