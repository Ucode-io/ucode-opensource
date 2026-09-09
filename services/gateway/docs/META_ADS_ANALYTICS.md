# Meta Ads Analytics

The gateway reads Meta Marketing API data on authenticated refresh requests and can serve the latest Redis snapshot first for stale-while-revalidate clients. It does not persist advertising data in PostgreSQL.

## Configuration

```env
META_GRAPH_BASE_URL=https://graph.facebook.com
META_GRAPH_VERSION=v26.0
META_AD_ACCOUNT_ID=843587364827107
META_ACCESS_TOKEN=<SYSTEM_USER_TOKEN>
META_ADS_CACHE_TTL_SEC=86400
META_ADS_REQUEST_TIMEOUT_SEC=30
META_ADS_MAX_RANGE_DAYS=366
META_LEAD_ACTION_TYPES=lead
META_ATTRIBUTION_WINDOWS=
```

`META_ACCESS_TOKEN` must be stored in backend secret storage. Do not add it to Git, frontend variables, request parameters, or logs. When Vault write access is unavailable, production CI may create a dedicated backend-only `/meta-ads.env` from the encrypted GitHub Actions secret. That file is present only in the private production image and is loaded after the runtime `/app/.env`. The account ID may be supplied with or without the `act_` prefix.

`META_LEAD_ACTION_TYPES` is an allowlist. Compare `observed_action_types` from a real 7–30 day response with Ads Manager before changing the production allowlist. When none of the configured lead actions is present, the Ucode Target landing-page funnel falls back to the account's `offsite_complete_registration_add_meta_leads` custom conversion. The fallback is ignored whenever a configured lead action is present, preventing historical rows that contain both action types from being counted twice. Leaving `META_ATTRIBUTION_WINDOWS` empty uses the ad account attribution setting.

## Endpoints

Both endpoints require the existing gateway authentication middleware.

```http
GET /v1/meta-ads/account
GET /v1/meta-ads/dashboard?since=2026-08-01&until=2026-08-20
GET /v1/meta-ads/dashboard?since=2026-08-01&until=2026-08-20&prefer_cache=true
```

Dashboard filters:

- `campaign_id`, `ad_set_id`, `ad_id`
- `objective`, `effective_status`
- `age`, `gender`, `country`, `region`
- `publisher_platform`, `platform_position`, `device_platform`
- `breakdowns=age_gender,country,region,placement,device`

Comma-separated values are accepted for every filter except object IDs. The default date range is the latest seven UTC dates and the maximum is controlled by `META_ADS_MAX_RANGE_DAYS`. Breakdowns are fetched only when explicitly requested because every breakdown is a separate Meta Insights request. `prefer_cache=true` returns the latest cached response immediately when one exists; clients should then repeat the request without that flag to refresh from Meta.

## Freshness and fallback

The dashboard uses stale-while-revalidate on the client: it first asks Redis for the last successful response with `prefer_cache=true`, renders it immediately, then repeats the request without the flag. The fresh request reads Meta directly and overwrites Redis after success. A successful fresh response contains:

```json
{"source":"meta","stale":false}
```

If Meta returns a transient error and a previous response exists, the gateway returns it as an explicit fallback:

```json
{"source":"cache","stale":true,"warning":"Meta API is temporarily unavailable; returning the last successful response"}
```

An intentional `prefer_cache=true` hit contains `{"source":"cache","stale":false}`. Cache keys contain only the public ad account ID and normalized dashboard filters; they never contain the access token.

## Response sections

- Account currency and timezone
- KPI totals: spend, impressions, reach, frequency, clicks, link clicks, landing page views, CTR, CPC, CPM, leads, and nullable CPL
- Daily trends
- Campaign → ad set → ad hierarchy with creative metadata
- Age/gender, country, region, placement, and device breakdowns
- Observed Meta action types for lead allowlist verification

Meta frequently returns numbers as strings. The gateway converts counts to integers, values to floating-point JSON numbers, and returns `cpl: null` when leads are zero. Personal Instant Form lead fields are outside this endpoint and require the existing Leads integration permissions.
