# Meta (Facebook) Lead Ads — спецификация интеграции

Документ описывает, **как мы подключаем Meta Lead Ads, где и что храним, откуда берём данные и как доводим лид до таблицы проекта**. Код по нему пишется отдельно; здесь — архитектура, контракты и план. Идентификаторы (поля, типы, эндпоинты) — на английском, прозой — по-русски.

Репозитории:
- `ucode_go_admin_api_gateway` (gateway) — OAuth-хендлеры, приём вебхука, проксирование в Graph API.
- `ucode_go_company_service` (company_service) — хранилище `project_resource`, миграции, proto, gRPC.

---

## 1. Глоссарий Meta

| Термин | Что это |
|---|---|
| **Page** | Страница Facebook. У неё есть `page_id` и **Page Access Token**. |
| **Lead Form** | Лид-форма, привязанная к Странице. `form_id`, набор `questions`. |
| **Lead** | Заполненная форма. `leadgen_id`, `field_data` (ответы). |
| **Page Access Token** | Токен Страницы. Им мы и подписываемся на вебхук, и тянем лид. |
| **leadgen webhook** | Глобальный вебхук приложения Meta; на каждый лид присылает `page_id + form_id + leadgen_id`. |

Ключевой факт: **вебхук один на всё приложение Meta**, в нём нет `project_id`. Привязку «лид → проект» мы делаем сами через `page_id`.

---

## 2. Модель хранения (company_service)

### 2.1. Одна строка `project_resource` = одна (Страница × проект × окружение)

- `type = 'META_LEADS'` (новое значение enum `resource_type`).
- Привязка скоупится `project_id + environment_id` (как все ресурсы).
- В одной Странице может быть **несколько лид-форм** — все маппинги форм лежат внутри `settings` этой строки (НЕ отдельными строками; формы не сваливаем в одну таблицу — у каждой формы своя целевая таблица).
- Одна и та же Страница может быть подключена к **нескольким проектам** → это **несколько строк** `project_resource` (по строке на проект). При входящем лиде раздаём его (fan-out) в каждый проект, у которого замаплена эта форма.

### 2.2. Новая колонка `external_id` (универсальный ключ резолва)

Сейчас в `project_resource` нет колонки для поиска по вторичному ключу, а `settings` JSONB по подстроке не индексируется. Добавляем **одну** генерик-колонку (переиспользуемую и для будущих интеграций):

```sql
-- migrations/postgres/NN_add_external_id_to_project_resource.up.sql
ALTER TABLE project_resource
    ADD COLUMN IF NOT EXISTS external_id VARCHAR NOT NULL DEFAULT '';

-- Уникальность НЕ глобальная: одна Страница на один проект/окружение = одна строка.
CREATE UNIQUE INDEX IF NOT EXISTS uq_project_resource_external_id
    ON project_resource (external_id, project_id, environment_id)
    WHERE external_id <> '';

-- Резолв вебхука по page_id идёт по этому индексу.
CREATE INDEX IF NOT EXISTS idx_project_resource_external_id
    ON project_resource (external_id) WHERE external_id <> '';
```

Для Meta: `external_id = page_id`.

> `ALTER TYPE resource_type ADD VALUE 'META_LEADS'` — отдельной миграцией. В Postgres `ADD VALUE` нельзя гонять внутри транзакции вместе с другими DDL — выносим в свой файл.

### 2.3. `Settings.FacebookLeadsCredentials` (proto)

Канонический proto — `ucode_protos/company_service/resource_service.proto`, message `Settings` (сейчас 10 полей, до `google_calendar = 10`). Добавляем новое поле.

> ⚠️ В `genproto` gateway поле 11 уже занято «висячим» `telegram` (drift: в исходном proto его нет). Перед генерацией **подтвердить свободный номер**; ниже используем `11` исходя из канонического источника — при необходимости заменить на следующий свободный.

```proto
message Settings {
    // ... существующие 1..10 ...
    FacebookLeadsCredentials facebook_leads = 11;
}

message FacebookLeadsCredentials {
    string page_id            = 1;  // == project_resource.external_id
    string page_name          = 2;
    string page_access_token  = 3;  // long-lived Page token (как Google refresh_token — в Settings открытым)
    string connected_user_id  = 4;  // ucode-юзер, подключивший
    string connected_at       = 5;  // RFC3339
    string status             = 6;  // active | revoked | error
    repeated FacebookLeadFormMapping forms = 7;  // по форме на элемент
}

// Маппинг одной лид-формы на одну таблицу проекта.
message FacebookLeadFormMapping {
    string form_id    = 1;
    string form_name  = 2;
    string table_slug = 3;  // целевая таблица в проекте
    repeated FacebookLeadFieldMapping fields = 4;
}

// Сопоставление одного поля формы Meta → полю таблицы ucode.
message FacebookLeadFieldMapping {
    string lead_field  = 1;  // ключ вопроса Meta (== field_data[].name), напр. "email", "phone_number", custom-key
    string table_field = 2;  // slug поля таблицы ucode
    bool   required    = 3;  // обязателен ли для создания записи (наша валидация)
}
```

### 2.4. Что и почему храним (обоснование набора полей)

- `page_access_token` — **обязателен**: только им тянется полный лид (`GET /{leadgen_id}`) и держится подписка. Берём long-lived токен Страницы, он не протухает, пока жив long-lived токен юзера.
- `page_id/page_name` — идентификация и UI «что подключено».
- `status` — для disconnect/инвалидации без удаления маппинга.
- `forms[]` — маппинги; вынесены внутрь Страницы, потому что лид приходит с `form_id`, и резолв «какая таблица» — по форме.
- `connected_user_id/connected_at` — аудит.

Чего **не** храним: короткоживущий user-token (нужен только в момент callback → обмена), client_secret (только в конфиге).

---

## 3. Резолв «page_id → проект(ы)» (company_service)

Новый gRPC + repo-метод, возвращающий **список** (для fan-out):

```proto
// resource_service.proto
rpc GetProjectResourcesByExternalId(GetByExternalIdRequest) returns (ListProjectResource) {}

message GetByExternalIdRequest { string external_id = 1; }
```

```go
// storage/postgres/resource.go
// WHERE external_id = $1  → может вернуть N строк (одна Страница на нескольких проектах)
SELECT id, project_id, environment_id, name, type, settings::text
FROM project_resource
WHERE external_id = $1 AND type = 'META_LEADS';
```

Резолв при вебхуке: `page_id → GetProjectResourcesByExternalId → [project_resource...]`. Для каждой строки в `settings.forms` ищем `form_id`; нет маппинга — пропускаем эту строку.

---

## 4. Поток подключения (gateway) — эндпоинты

Группа `/v1/facebook` (auth), callback публичный. Шаблон — Google Drive (`api/handlers/v1/google_drive.go`): random `state` → Redis (TTL 10 мин) → callback достаёт контекст.

| Метод | Путь | Auth | Назначение |
|---|---|---|---|
| GET | `/v1/facebook/connect` | ✅ | строит Facebook Login URL, кладёт `state{project_id,environment_id,user_id,frontend_origin}` в Redis |
| GET | `/v1/facebook/callback` | — (public) | `code` → user token → **long-lived** token; кладёт его в Redis под `state`; редирект на `frontend_origin` |
| GET | `/v1/facebook/pages` | ✅ | `GET /me/accounts` → список Страниц (`id,name,access_token`) для выбора |
| GET | `/v1/facebook/pages/:page_id/forms` | ✅ | `GET /{page_id}/leadgen_forms` → формы Страницы (для шага маппинга) |
| GET | `/v1/facebook/forms/:form_id/questions` | ✅ | `GET /{form_id}?fields=questions` → вопросы формы (фронт строит маппинг) |
| POST | `/v1/facebook/subscribe` | ✅ | юзер выбрал Страницу: `POST /{page_id}/subscribed_apps?subscribed_fields=leadgen` + создаём `project_resource(external_id=page_id, settings.page_*)` |
| PUT | `/v1/facebook/mapping` | ✅ | сохранить/обновить `settings.forms[]` (маппинг форма→таблица→поля) |
| GET | `/v1/facebook/integration` | ✅ | **status**: подключено? какая Страница, какие формы замаплены |
| GET | `/v1/facebook/integration/validate` | ✅ | проверка живости page-токена (Graph `GET /{page_id}?fields=id`) |
| DELETE | `/v1/facebook/integration/:id` | ✅ | **disconnect**: `DELETE /{page_id}/subscribed_apps` + удалить строку `project_resource` |

Конфиг (`BaseConfig`): `FacebookAppID`, `FacebookAppSecret`, `FacebookRedirectURI`, `FacebookFrontendSuccessURL`, `FacebookFrontendErrorURL`, `FacebookGraphBaseURL` (`https://graph.facebook.com`) + версия (`v21.0`). `FacebookWebhookVerifyToken` уже есть. `AppSecret` также используется для проверки подписи вебхука.

**OAuth scopes:** `leads_retrieval`, `pages_show_list`, `pages_manage_metadata`, `pages_read_engagement`. (Для прод-доступа Meta требует business verification + Advanced Access на `leads_retrieval`.)

---

## 5. Приём и доведение лида (gateway) — end-to-end

Эндпоинты приёма уже есть (`api/handlers/v1/facebook.go`):
- `GET /webhook/facebook` — verify (echo `hub.challenge`). ✅ работает.
- `POST /webhook/facebook` — сейчас просто `{ok:true}`.

Целевой поток `POST /webhook/facebook`:

1. **Проверить подпись** `X-Hub-Signature-256` = `HMAC-SHA256(body, AppSecret)` (constant-time compare, как Telegram-secret / Stripe). Невалидно → `200` без обработки (Meta не должна ретраить, но и данные не трогаем).
2. Ответить `200 {ok:true}` **сразу** (Meta требует ответ ≤ ~20 сек), дальше — в фоне.
3. Для каждого `entry`: `page_id = entry.id`; для каждого `change` с `field=="leadgen"`: взять `value.leadgen_id`, `value.form_id`.
4. `GetProjectResourcesByExternalId(page_id)` → список строк.
5. Для каждой строки: найти в `settings.forms` элемент с `form_id`. Нет — пропустить (этот проект форму не маппил).
6. **Дотянуть лид:** `GET /{leadgen_id}?access_token={page_access_token}&fields=id,created_time,ad_id,ad_name,campaign_id,campaign_name,form_id,platform,is_organic,field_data`.
7. Превратить `field_data[]` (`{name, values[]}`) в запись по `fields[]` маппинга (`lead_field → table_field`); проверить `required`.
8. **Записать в таблицу** `table_slug` проекта через object_builder (как `CreateObject`). Дедуп по `leadgen_id` (рекомендуется поле-ключ в таблице).

> Шаги 6–8 (дотяжка + запись) — **в этих же задачах пишем сами**, не откладываем.

**Идемпотентность:** Meta может прислать лид повторно. Храним `leadgen_id` в записи и при вставке проверяем дубликат.

---

## 6. Справочник Meta API (для backend и frontend)

### 6.1. Вебхук leadgen (тело POST)
```json
{
  "object": "page",
  "entry": [{
    "id": "<PAGE_ID>",
    "time": 1700000000,
    "changes": [{
      "field": "leadgen",
      "value": {
        "leadgen_id": "<LEAD_ID>",
        "page_id": "<PAGE_ID>",
        "form_id": "<FORM_ID>",
        "adgroup_id": "<AD_ID>",
        "ad_id": "<AD_ID>",
        "created_time": 1700000000
      }
    }]
  }]
}
```
(Наши модели `api/models/facebook.go` уже это покрывают.)

### 6.2. Лид-объект (`GET /{leadgen_id}`)
Поля: `id`, `created_time`, `ad_id`, `ad_name`, `adset_id`, `adset_name`, `campaign_id`, `campaign_name`, `form_id`, `platform`, `is_organic`, **`field_data`**.
```json
{
  "id": "<LEAD_ID>",
  "created_time": "2026-01-01T10:00:00+0000",
  "ad_id": "...", "form_id": "...", "platform": "fb",
  "field_data": [
    { "name": "full_name",    "values": ["Иван Петров"] },
    { "name": "email",        "values": ["ivan@example.com"] },
    { "name": "phone_number", "values": ["+998901234567"] },
    { "name": "what_car?",    "values": ["Tesla"] }   // custom-вопрос
  ]
}
```
`field_data[].name` = **`key`** соответствующего вопроса формы. Для стандартных полей — предопределённые ключи; для кастомных — ключ кастом-вопроса.

### 6.3. Форма (`GET /{form_id}?fields=questions,name,status,locale`)
Поля формы: `id`, `name`, `status`, `locale`, `privacy_policy{url,link_text}`, `questions[]`.
Вопрос: `{ key, label, type, options[] }`.
Типы (`type`), которые видит фронт:
- Контакты: `FULL_NAME, FIRST_NAME, LAST_NAME, EMAIL, PHONE, WORK_EMAIL, WHATSAPP_NUMBER`
- Адрес: `STREET_ADDRESS, CITY, STATE, PROVINCE, ZIP, POST_CODE, COUNTRY`
- Демография: `DOB, GENDER, MARITAL_STATUS`
- Работа: `COMPANY_NAME, JOB_TITLE, WORK_PHONE_NUMBER`
- Прочее: `CUSTOM` (произвольный вопрос; ключ — слаг лейбла), `DATE_TIME`, `SLIDER`, региональные ID.

### 6.4. Токены и подписка
- Facebook Login → user token → обмен на **long-lived** (`GET /oauth/access_token?grant_type=fb_exchange_token`).
- `GET /me/accounts` → `[{ id, name, access_token, tasks }]` — это и есть Page-токены.
- Подписка Страницы: `POST /{page_id}/subscribed_apps` body `subscribed_fields=leadgen` (page-токеном).
- Отписка: `DELETE /{page_id}/subscribed_apps`.

---

## 7. Контракт для фронта (как строить маппинг)

Цель фронта: по каждой форме собрать `table_slug` + список `lead_field → table_field`.

Шаги UI:
1. `GET /v1/facebook/pages` → юзер выбирает Страницу.
2. `POST /v1/facebook/subscribe {page_id}` → подключили Страницу.
3. `GET /v1/facebook/pages/:page_id/forms` → список форм.
4. Для выбранной формы: `GET /v1/facebook/forms/:form_id/questions` → массив `{key, label, type}`. **Это и есть поля лида** — слева в маппинге.
5. Параллельно фронт берёт поля целевой таблицы ucode (существующий эндпоинт схемы таблицы) — справа.
6. Юзер сопоставляет; помечаем `required`.
7. `PUT /v1/facebook/mapping` с `{ page_id, forms:[{form_id, table_slug, fields:[{lead_field, table_field, required}]}] }`.

Рекомендации по «обязательности» (наша валидация, не Meta):
- **Обязательно замаппить:** `table_slug` и хотя бы один идентификатор контакта — `email` **или** `phone_number`. Без идентификатора лид бесполезен/недедуплицируется.
- **Рекомендуется:** `full_name` (или `first_name`+`last_name`).
- Системное: завести в таблице поле под `leadgen_id` (наш ключ дедупликации) — маппится автоматически, скрыто.
- Кастом-вопросы (`type=CUSTOM`) — по желанию; `lead_field` = их `key`.

Важно для фронта: **`lead_field` — это `key` вопроса, а НЕ `label`.** Лейбл может быть на любом языке/меняться; `key` стабилен и совпадает с `field_data[].name`.

### Решение по необработанным лидам (утверждено)
**Лид по форме без маппинга** (нет элемента с этим `form_id` в `settings.forms[]`) — **пропускаем и логируем**; вебхук всё равно отвечает Meta `200`. «Сырьём» в дефолтную таблицу НЕ складываем. То же для неизвестного `lead_field` внутри замапленной формы — поле игнорируем, лид пишем по остальным маппингам (если выполнены `required`).

---

## 8. Что уже сделано

- `GET/POST /webhook/facebook` зарегистрированы; verify работает (`api/handlers/v1/facebook.go`).
- `config.FacebookWebhookVerifyToken` (`config/config.go`).
- Модели тела вебхука (`api/models/facebook.go`).

---

## 9. План работ (этапы)

**company_service (proto/БД/gRPC — генерацию proto делает владелец):**
1. Миграция: `external_id` + индексы; новый enum `META_LEADS`.
2. proto: `FacebookLeadsCredentials` + `FacebookLeadFormMapping` + `FacebookLeadFieldMapping` в `Settings`; RPC `GetProjectResourcesByExternalId`.
3. storage/repo: запись/чтение `external_id` в `AddResourceToProject`/`UpdateProjectResource`/`GetSingle`; метод `GetProjectResourcesByExternalId`.
4. gRPC service: проброс нового RPC.

**gateway:**
5. Конфиг: `FacebookAppID/Secret/RedirectURI/Frontend*Url/GraphBaseURL`.
6. OAuth: `connect` + `callback` (шаблон Google Drive, state→Redis).
7. `pages`, `pages/:id/forms`, `forms/:id/questions` (проксирование Graph).
8. `subscribe` (+ создание `project_resource`), `PUT mapping`.
9. `integration` (status), `integration/validate`, `DELETE integration/:id` (disconnect + отписка).
10. Вебхук: проверка `X-Hub-Signature-256`; резолв `page_id`→проекты; дотяжка лида; запись в таблицу по маппингу + дедуп по `leadgen_id`.

Порядок: 1–4 (company_service) разблокируют 8 и 10. Параллельно 5–7 в gateway. Затем 8–9, в конце 10.

> Кросс-репо: правки proto — в `ucode_protos` (генерацию/пуш делает владелец). gateway-хендлеры — внутри gateway.
