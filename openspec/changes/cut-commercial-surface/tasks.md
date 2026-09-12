# Tasks

## 1. Pricing data

- [x] 1.1 Strip the seeded fares, fare items, fare item prices and billing
      periods from migrations 47, 68, 70, 72 and 77. The table definitions stay
      so a fork that wants billing has somewhere to put it
- [x] 1.2 Drop `DEFAULT 'monthly'` from `subscription.billing_period_code`.
      It references `ugen_billing_period`, which is now empty, so a defaulted
      insert would have failed the foreign key
- [x] 1.3 Confirm the migrations still apply — all 90 ran clean against a fresh
      Postgres, and the pricing tables came out empty while `currency` (119
      rows) and `language` (184) survived
- [x] 1.4 Confirm the platform tolerates the empty tables. Ran both real query
      shapes from `project.go` against the stripped schema with a project row
      present: the row comes back, `subscription_info` and `fare_name` NULL
- [x] 1.5 Rebuild the migrate image and confirm no price survives in it
- [ ] 1.6 Verify on a genuinely fresh install. Everything above was checked
      against a throwaway database; `ucode reset && ucode start && make smoke`
      is the real proof, and it destroys whatever is in the local stack, so it
      is the owner's call when to run it
- [ ] 1.7 Decide what to do about the already-published images

## 2. Ugen

- [ ] 2.1 auth: the five `/v3/ugen/*` routes, `ugent_auth.go`, `UgenLogin` and
      `ugenLoginForAuthenticatedUser`
- [ ] 2.2 gateway: the four `/v1/ugen/*` routes, `handlers/v1/ugen.go`,
      `models/ugen.go`
- [ ] 2.3 company: `ugen_template` storage, grpc service and repo
- [ ] 2.4 Drop `is_ugen` from the refresh-login response. It costs two gRPC
      round-trips per refresh to compute a field the admin panel never reads
- [ ] 2.5 proto: delete `ugen_template.proto`, strip the ugen messages from
      `projects_service`, `session_service`, `company_service` and
      `resource_service`, regenerate, and diff the generated output before and
      after — the same way the billing protos were done
- [ ] 2.6 Schema: `ugen_template`, `ugen_template_reaction`,
      `ugen_billing_period`, `ugen_currency_rate_cache`, the `is_ugen` column
      and the `product_type` enum
- [ ] 2.7 Rebuild, run the unit tests and walk the smoke path
