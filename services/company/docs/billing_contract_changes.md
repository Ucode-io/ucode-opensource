# Billing contract changes (for frontend)

This note covers the user-visible behaviour changes from the billing/cron rework.
The earlier changes touched only the **error responses** and **balance semantics** of
two existing RPCs; a later addition introduces one **new read-only RPC**
(`GetProjectBillingStatus`, §4). Everything else is internal (cron scheduling,
idempotency).

---

## 1. `AttachFare` (ugen self-serve plan changes)

`AttachFare` returns the underlying error directly, so over gRPC these arrive with
status code **`Unknown`** and the message string below. Key on the **message**, not
the code.

| Situation | Message |
|---|---|
| Re-attaching the plan the project is **already on** (active period) | `project is already subscribed to this plan` |
| Switching between **ucode and ugen** product types (either direction) | `cannot switch between ucode and ugen plans` |
| Calling `AttachFare` for a **ucode** plan | `ucode plans cannot be changed via AttachFare` |
| Attaching while the current sub is **scheduled to cancel** at period end | `subscription is scheduled to cancel at period end` |
| `balance + credit_limit` cannot cover the charge | `balance + credit limit is less than fare price` |

What changed:

- **Same-plan re-attach is now rejected** instead of silently renewing/charging.
  Previously, re-attaching the active plan could double-charge; renewals are owned
  by the cron, so the API now refuses it with `project is already subscribed to this plan`.
- **Cross-product guard now also covers the "initial" path.** A project on a real
  (non `free_trial`) ucode subscription can no longer be flipped onto a ugen plan
  via the initial code path — it returns `cannot switch between ucode and ugen plans`.
  A `free_trial` subscription is still product-neutral and may convert to ugen.

Frontend guidance: surface these as friendly, non-retryable messages. The "already
subscribed" case is a no-op — treat it as success-equivalent in the UI (the user is
already on that plan), don't show a hard error.

---

## 2. `CreateTransaction`

Two behaviour changes; the request/response shapes are unchanged.

1. **Credit limit is now honoured.** A `subscription` (debit) transaction is allowed
   when `balance + credit_limit >= amount`. Previously it was rejected whenever
   `balance < amount`, ignoring the project's overdraft allowance.
2. **Insufficient funds is now a typed error.** When the charge can't be covered, the
   RPC returns gRPC status **`FailedPrecondition`** with message
   `balance + credit limit is less than fare price` (previously a generic error).
3. **Atomicity.** The transaction row and the balance update now happen in one DB
   transaction under a row lock, so a topup and a debit can no longer race. There is
   no API-shape change here, but a successful response now guarantees the balance was
   moved.

Frontend guidance: map `FailedPrecondition` on `CreateTransaction` to a "top up your
balance" prompt. `topup` transactions are unaffected and always credit the balance.

---

## 3. Renewals (no API surface, FYI)

ucode renewals run on a `*/2 * * * *` tick. If a project can't afford its renewal,
its `status` becomes `insufficient_funds` while the subscription stays `active`; the
charge is retried automatically on the next tick or immediately after a successful
topup. No client action beyond topping up is required to recover.

---

## 4. `GetProjectBillingStatus` (new — billing status + low-balance warning)

New **read-only** RPC on `BillingService`. Exposed through the admin gateway as:

```
GET /v1/billing/status      (ApiKeyAuth; project resolved from the auth context)
```

Returns the project's billing health. `project_status` is always present; the warning
fields are filled only when the project is close to a renewal it cannot afford.

| field | meaning |
|---|---|
| `project_status` | `active` / `insufficient_funds` / `blocked` / `inactive` / `pending` |
| `subscription_status` | current subscription status (`active` / `pending_downgrade` / …), empty if none |
| `product_type` | `ucode` / `ugen` |
| `renewal_date` | date of the next charge (`YYYY-MM-DD`), empty if no paid subscription |
| `days_until_renewal` | whole days from today to `renewal_date` (negative if overdue) |
| `next_charge` | amount of the next charge, in UZS (FX already applied) |
| `project_balance`, `credit_limit` | current balance and overdraft allowance, UZS |
| `available_funds` | `project_balance + credit_limit` |
| `low_balance_warning` | `true` iff `days_until_renewal <= 10` **and** `available_funds < next_charge` |
| `shortfall` | `next_charge - available_funds` when the warning is set, else `0` |
| `currency_code` | fare currency (`UZS` / `USD`) |

`next_charge` mirrors what the renewal will actually charge: for **ucode** it is one
calendar month of the fare; for **ugen** it is `price × billing_period_months ×
(1 − period_discount)`. Both are converted to UZS at the current rate. Free-tier
projects (`next_charge = 0`) and projects without a paid subscription never warn.

Frontend guidance: read this on the billing screen. When `low_balance_warning` is
`true`, show a "top up before `renewal_date`" banner with the `shortfall` amount;
otherwise just reflect `project_status`. Already-blocked projects
(`insufficient_funds` / `blocked`) should keep showing the top-up CTA regardless of
the warning flag.
