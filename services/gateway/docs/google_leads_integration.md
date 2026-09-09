# Google Lead Form Ads — спецификация интеграции

> Статус: план (утверждён к реализации). Аналог Meta Lead Ads, но **webhook-only** —
> без OAuth, без Google Ads API. Архитектура опирается на уже построенную
> инфраструктуру Meta (`docs/meta_leads_integration.md`): колонка `external_id` в
> `project_resource`, RPC `GetProjectResourcesByExternalId`, fan-out, дедуп.

## 0. Принятые решения (зафиксировано с пользователем)

| Вопрос | Решение |
|---|---|
| Подход | **Webhook-only** — приём лидов через вебхук + `google_key`. Без OAuth, без Google Ads API. |
| Ключ резолва (`external_id`) | **Сгенерённый `google_key`** — уникальный секрет на ресурс. Одновременно auth и ключ маршрутизации. |
| Маппинг полей | **Руками заранее** — юзер выбирает `column_id` (стандартный список + кастомные) → колонка таблицы, до первого лида. |
| Тестовые лиды (`is_test=true`) | **Пишем в таблицу** наравне с боевыми (дедуп по `lead_id` защищает от дублей). |

---

## 1. Глоссарий Google

- **Lead Form Ad / Lead form asset** — форма заявки внутри объявления Google (Search/YouTube/Display/Discovery). Аналог Meta Lead Ad.
- **`lead_id`** — уникальный ID заявки. Используем для дедупа (аналог `leadgen_id` у Meta).
- **`form_id`** — ID лид-формы в Google Ads.
- **`google_key`** — секретная строка, **которую генерим МЫ**. Юзер вписывает её (вместе с нашим Webhook URL) в настройки лид-формы в Google Ads. Google возвращает её в каждом payload — мы по ней и аутентифицируем запрос, и находим проект.
- **`user_column_data[]`** — массив ответов лида: `{column_id, string_value, column_name}`.
- **`column_id`** — код поля Google. Стандартные: `FULL_NAME`, `FIRST_NAME`, `LAST_NAME`, `EMAIL`, `PHONE_NUMBER`, `POSTAL_CODE`, `STREET_ADDRESS`, `CITY`, `REGION`, `COUNTRY`, `COMPANY_NAME`, `JOB_TITLE`, `WORK_EMAIL`, `WORK_PHONE`; плюс кастомные вопросы рекламодателя со своими id.

### Чем принципиально отличается от Meta

| | **Meta (PULL)** | **Google (PUSH)** |
|---|---|---|
| Что в вебхуке | только `leadgen_id` | **все данные лида сразу** |
| Второй запрос за данными | да (`GET /{leadgen_id}` + page-token) | **нет** |
| OAuth | нужен (user-token, page-token) | **не нужен** |
| Аутентификация вебхука | HMAC-SHA256 (`X-Hub-Signature-256`) | общий секрет `google_key` в теле |
| Кто настраивает связь | мы через API (`me/accounts`) | рекламодатель **руками** в Google Ads UI |
| Список полей формы | тянем через API (`/forms/questions`) | юзер маппит руками (API не дёргаем) |

Вывод: у Google выпадают целиком connect/callback, токены, дотяжка лида и
HMAC — остаётся ядро **резолв → маппинг → запись → дедуп**.

---

## 2. Модель хранения (company_service)

### 2.1. Одна строка `project_resource` = одна (лид-форма × проект × окружение)

- `type = 'GOOGLE_LEADS'` (новое значение enum `resource_type`).
- `external_id = google_key` (генерим при создании ресурса). Резолв вебхука по нему.
- UNIQUE-индекс `(external_id, project_id, environment_id)` **уже существует** (создан миграцией 83 под Meta) → переиспользуем. Колонку `external_id` **не добавляем повторно**.
- Один и тот же `google_key` теоретически можно завести на нескольких проектах → лид раздаётся (fan-out) в каждый. На практике ключ уникален per-resource.

### 2.2. `Settings.GoogleLeadsCredentials` (proto, новое)

```proto
// company.proto
enum ResourceType {
    // ...
    META_LEADS = 18;
    INSTAGRAM = 19;      // <-- teammate (не трогаем)
    GOOGLE_LEADS = 20;   // <-- новое (additive; сдвинуто с 19 из-за конфликта с INSTAGRAM)
}
```

```proto
// resource_service.proto
message Settings {
    // ... facebook_leads = 12;
    InstagramCredentials instagram = 13;       // <-- teammate (не трогаем)
    GoogleLeadsCredentials google_leads = 14;   // <-- новое (additive; сдвинуто с 13)
}

// Google Lead Form Ads (webhook-only). external_id of project_resource holds the
// generated google_key — it is both the webhook auth secret and the routing key.
// No OAuth/tokens: Google pushes full lead data to our webhook.
message GoogleLeadsCredentials {
    string google_key  = 1;   // generated secret; mirrored into external_id
    string form_id     = 2;   // optional: validate incoming form_id ("" = accept any)
    string form_name   = 3;
    string table_slug  = 4;
    string status      = 5;
    string connected_at = 6;
    repeated GoogleLeadFieldMapping fields = 7;
}

// One Google lead column mapped to one ucode table field. lead_column == the Google
// column_id (e.g. FULL_NAME, EMAIL), not the human label.
message GoogleLeadFieldMapping {
    string lead_column  = 1;   // == user_column_data[].column_id
    string table_field  = 2;
    bool   required     = 3;
}
```

Замечания по дизайну:
- **Одна форма = один ресурс = один `google_key`**, поэтому маппинг плоский
  (`table_slug` + `fields[]`), без вложенного `forms[]` как у Meta. Это возможно
  именно потому, что `google_key` уже 1:1 с маппингом (у Meta page→много forms).
- `form_id` опционален: если задан — проверяем совпадение с payload (защита от
  чужой формы на том же ключе); пусто — принимаем любой `form_id`.

### 2.3. Что НЕ нужно в company_service

- Новой RPC нет — `GetProjectResourcesByExternalId` уже принимает `type` и подходит.
- Новой колонки/индекса нет (есть с Meta).
- storage/repo `project_resource` работают с `settings` (jsonb) генерически → правок нет.
- Единственная миграция — `ALTER TYPE resource_type ADD VALUE 'GOOGLE_LEADS'`.

---

## 3. Поток подключения (gateway) — эндпоинты

Все, кроме вебхука, под `AuthMiddleware`. Группа `/v1/google-leads`.

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/webhook/google` | **public.** Приём лида от Google. |
| `GET` | `/v1/google-leads/columns` | Справочник стандартных `column_id` (для UI маппинга). |
| `POST` | `/v1/google-leads` | Создать интеграцию: генерим `google_key`, сохраняем маппинг, возвращаем `google_key` + Webhook URL. |
| `PUT` | `/v1/google-leads/mapping/:id` | Обновить маппинг полей/таблицы. |
| `GET` | `/v1/google-leads/integration` | Список интеграций проекта (+ маппинг, статус). |
| `DELETE` | `/v1/google-leads/integration/:id` | Удалить интеграцию. |

**Что видит юзер при создании:** мы отдаём ему `google_key` и Webhook URL —
он копирует обе строки в Google Ads UI (лид-форма → Delivery options).

---

## 4. Приём лида (gateway) — end-to-end

### 4.1. Структура payload (POST на `/webhook/google`)

```json
{
  "lead_id": "f9a8...e1",
  "api_version": "1.0",
  "form_id": "123456789",
  "campaign_id": "987654321",
  "gcl_id": "Cj0KCQ...",
  "adgroup_id": "111",
  "creative_id": "222",
  "is_test": true,
  "google_key": "сгенерённый-нами-секрет",
  "user_column_data": [
    { "column_id": "FULL_NAME",    "string_value": "John Doe",      "column_name": "Full name" },
    { "column_id": "EMAIL",        "string_value": "john@mail.com",  "column_name": "Email" },
    { "column_id": "PHONE_NUMBER", "string_value": "+99890...",      "column_name": "Phone" }
  ]
}
```

### 4.2. Алгоритм

```
POST /webhook/google
  1. Прочитать тело, распарсить JSON. Любая ошибка → 200 {ok:true} (Google ретраит на не-200).
  2. google_key пустой → 200 (тихо игнор).
  3. Быстрый ack: вернуть 200 СРАЗУ, обработку — в горутину с defer recover()
     (как processFacebookLeadEvent). is_test НЕ влияет на ack.
  4. (в горутине) GetProjectResourcesByExternalId(google_key, GOOGLE_LEADS) → resources[].
  5. resources пуст → log.Warn("no project mapped"), выход.
  6. Для каждого resource (fan-out):
       cred := resource.Settings.GoogleLeads
       cred == nil → continue
       cred.form_id != "" && cred.form_id != payload.form_id → log + continue (чужая форма)
       constant-time сравнить cred.google_key с payload.google_key → не совпало → log + continue
       values := map[column_id]string_value из user_column_data
       data := {}
       для каждого field в cred.fields:
           raw := values[field.lead_column]
           raw == "" && field.required → log.Error("required column missing"), continue/skip строки
           raw != "" → data[field.table_field] = raw
       data пуст → log + continue
       data["guid"] = uuid.NewSHA1(NameSpaceOID, "google-lead:"+lead_id)   // дедуп
       ObjectBuilder().Create(table_slug=cred.table_slug, data) (как writeFacebookLead)
```

- **`is_test=true` пишем в таблицу** (решение пользователя). Дедуп по `lead_id`
  гарантирует, что повторный тестовый лид не задвоится.
- Аутентификация — `google_key` (constant-time compare через `crypto/subtle`).
  HMAC не нужен.
- Всегда отвечаем Google **200**, чтобы не было ретраев.

### 4.3. Маппинг значений

Из `user_column_data` строим `map[column_id] = string_value`, затем по
`cred.fields` раскладываем в `data[table_field]`. **Заглушки `// TEST:` как в
Meta НЕ требуется** — данные приходят в payload, никакой дотяжки нет.

---

## 5. Конфигурация (config.go)

OAuth/AppSecret/Graph — **не нужны**. Опционально:

- `GOOGLE_LEADS_WEBHOOK_URL` — публичный URL вебхука (например
  `https://api.../webhook/google`), чтобы отдавать его юзеру в ответе на create.
  Если не задан — можно собрать из существующего base-host конфига.

Новых обязательных env-переменных нет.

---

## 6. Контракт для фронта

1. `GET /v1/google-leads/columns` → список стандартных `column_id` (для дропдауна),
   плюс поле «свой column_id» для кастомных вопросов.
2. Юзер выбирает таблицу (`table_slug`) и строит маппинг `column_id → table_field`
   (+ флаг `required`), опционально вписывает `form_id`/`form_name`.
3. `POST /v1/google-leads` → ответ `{ google_key, webhook_url, resource_id }`.
4. UI показывает инструкцию: «Скопируй `webhook_url` и `google_key` в Google Ads →
   твоя лид-форма → Delivery options (вебхук)».
5. Правки маппинга — `PUT /v1/google-leads/:id/mapping`.

---

## 7. План работ (этапы)

### Phase A — proto + миграция (репо `ucode_go_company_service`, домен пользователя)
1. `company.proto`: `GOOGLE_LEADS = 20` в enum `ResourceType` (19 занял `INSTAGRAM` тиммейта).
2. `resource_service.proto`: `GoogleLeadsCredentials google_leads = 14` в `Settings` (13 занял `instagram`)
   + messages `GoogleLeadsCredentials`, `GoogleLeadFieldMapping`.
3. Те же правки в `ucode_protos` обоих репо (submodule — истинный источник).
4. Миграция `86_add_google_leads.up.sql` (85 занял teammate `add_instagram_project_resource`):
   ```sql
   ALTER TYPE resource_type ADD VALUE IF NOT EXISTS 'GOOGLE_LEADS';
   ```
   (`external_id` + индексы уже есть из миграции 83.) `.down.sql` — no-op коммент
   (значения enum в PG не удаляются безопасно).
5. **Пользователь сам** запускает `make gen-proto-module` в обоих репо (codegen
   НЕ запускаем мы) и проверяет `go build ./...`.

### Phase B — gateway (`ucode_go_admin_api_gateway`)
1. `api/models/google_leads.go` — payload вебхука + request/response контракты.
2. `api/handlers/v1/google_leads.go`:
   - `GoogleWebhookReceive` (ack + горутина + recover, резолв, маппинг, запись, дедуп).
   - `GoogleLeadsCreate` (генерация `google_key`, upsert ресурса).
   - `GoogleLeadsSaveMapping`, `GoogleLeadsIntegration`, `GoogleLeadsDisconnect`,
     `GoogleLeadsColumns`.
   - Хелперы конвертации proto↔models (по образцу `facebookFormsToProto`).
3. Роуты в `api/api.go` (public `/webhook/google` + группа `/v1/google-leads`).
4. (опц.) `config.go`: `GOOGLE_LEADS_WEBHOOK_URL`.
5. `go build ./...` + `go vet` зелёные.

### Не делаем (в отличие от Meta)
OAuth connect/callback, page/user-токены, `me/accounts`, дотяжку лида, HMAC,
тестовую заглушку `// TEST:`.

---

## 8. Безопасность

- `google_key` — секрет, сравнение constant-time (`crypto/subtle`).
- Эндпоинт только по HTTPS (требование Google).
- Вебхук всегда отвечает 200 (даже при ошибке/невалидном payload), обработка — вне ack.
- Дедуп по `lead_id` (`guid = SHA1("google-lead:"+lead_id)`).
- Горутина обработки с `defer recover()` — кривой payload не уронит gateway.
</content>
</invoke>
