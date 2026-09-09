# Handoff — Платные ugen-шаблоны (полный контекст сессии)

> Этот файл — **полный контекст** завершённой работы по фиче «платные шаблоны». Читай его в новом
> чате, чтобы понять что уже сделано, какие решения приняты, что где лежит и что осталось.
> Дата работы: 2026-07-02.

---

## 0. TL;DR

Добавили в ugen-шаблоны **цену** и **списание с баланса** при создании проекта из шаблона.
- **Юзер** создаёт шаблон **бесплатно** (цену задать не может).
- **Админ** задаёт цену отдельным эндпоинтом (как у token packs).
- При «использовать шаблон» (`create-project`): если `price>0` → атомарно списываем с баланса
  **текущего (head) проекта**; при провале провижининга — **авто-возврат**. Free (0) → без биллинга.
- Заодно **переписали `CreateProjectFromTemplate`**: убрали фолбэки/мусор и починили источники данных
  (был баг `project not found`).

Статус: **код готов**, ждёт только **регенерации proto** пользователем в обеих репах.

---

## 1. Репозитории (go-workspace, соседние папки)

- `ucode_go_admin_api_gateway` — API-шлюз (Gin + gRPC-клиенты). Основные хендлеры.
- `ucode_go_company_service` — биллинг + хранилище шаблонов (Postgres). Атомарное списание.
- `ucode_go_object_builder_service` — tenant-БД (таблицы/данные/mcp_project/микрофронты). Только читали.
- Proto — общий модуль `ucode_protos` (есть копия в каждой репе, синхронизируются). Rsync-копия — `protos/`
  (её НЕ трогаем). Генерацию protoc делает **пользователь сам** (`make copy-proto-module && make gen-proto-module`).

---

## 2. Принятые продуктовые решения

1. **Кто платит**: текущий (head) проект из контекста запроса (единственный с балансом; target-проект стартует с 0).
2. **При провале после списания**: авто-refund (компенсирующая транзакция), best-effort с логом.
3. **Модель цены**: `price` (double) на шаблоне, `0 = бесплатно`; `currency_id` (uuid, пусто = UZS).
4. **Цену задаёт только админ** отдельным эндпоинтом (не в create/update). Юзер цену передать не может —
   полей цены в create/update **нет** в proto.
5. **Идемпотентность**: `idempotency_key` в create-project (external_id в транзакции); refund keyed `<charge_id>:refund`.
6. Баланс хранится в **UZS**; цена конвертируется по курсу валюты (`fetchUgenCurrencyRate`), как у token packs.

---

## 3. Изменения по файлам

### 3.1. Proto (`ucode_protos/company_service/`, синхронно в ОБЕИХ репах)

**`ugen_template.proto`:**
- `UgenTemplate`: добавлены `double price = 22;` `string currency_id = 23;`
- `CreateUgenTemplateReq` / `UpdateUgenTemplateReq`: **БЕЗ полей цены** (юзер цену не задаёт).
- Новый RPC: `rpc SetPrice(SetUgenTemplatePriceReq) returns (UgenTemplate)`.
- Новое сообщение: `SetUgenTemplatePriceReq { string id = 1; double price = 2; string currency_id = 3; }`.

**`billing_service.proto`:**
- Новые RPC: `ChargeProjectBalance` и `RefundProjectBalance`.
- Сообщения: `ChargeProjectBalanceRequest{project_id, amount, currency_id, creator_id, comment, transaction_type, external_id}`,
  `ChargeProjectBalanceResponse{transaction_id, charged_amount, project_balance}`,
  `RefundProjectBalanceRequest{project_id, transaction_id, comment}`,
  `RefundProjectBalanceResponse{transaction_id, refunded_amount, project_balance}`.
- ✅ Проверено `protoc` parse (exit 0) — реген пройдёт чисто.

### 3.2. company_service (`ucode_go_company_service`)

- **Миграция `migrations/postgres/87_add_ugen_template_pricing.{up,down}.sql`**:
  - `ALTER TABLE ugen_template ADD price DECIMAL(20,2) NOT NULL DEFAULT 0, ADD currency_id UUID REFERENCES currency(id)`.
  - `ALTER TYPE transaction_type ADD VALUE 'template_purchase' / 'template_refund'` (в DO-блоке, как миграция 84 token packs).
  - Индекс `idx_transaction_external_id` (partial, external_id<>''). Колонки `external_id`/`rate` уже были (миграция 58).
- **`config/constants.go`**: `TransactionTypeTemplate = "template_purchase"`, `TransactionTypeTemplateRfnd = "template_refund"`.
- **`config/errors.go`**: `ErrInvalidChargeAmount`, `ErrProjectNotFound`, `ErrTransactionNotFound`, `ErrTransactionNotRefundable`.
- **`storage/postgres/ugen_template.go`**:
  - Create/Update **не пишут** цену (Create → дефолт 0; Update не трогает). Цену **читают** в ответ.
  - Новый метод **`SetPrice`** (UPDATE только price+currency_id, возвращает полный шаблон).
- **`storage/postgres/ugen_template_billing.go`** (новый файл): `ChargeProjectBalance`, `RefundProjectBalance`,
  `resolveCurrencyCode`. Атомарно: `SELECT ... FOR UPDATE` на project, проверка `balance+credit_limit >= amountUZS`,
  дебет, INSERT transaction. Идемпотентность по external_id (сравнение `transaction_type::text = $3`).
  Refund — компенсирующая транзакция. По образцу `PurchaseTokenPack` в `token_pack.go`.
- **`storage/repo/ugen_template.go`** + **`storage/repo/billing.go`**: добавлены методы в интерфейсы.
- **`grpc/service/ugen_template.go`**: метод `SetPrice`.
- **`grpc/service/billing.go`**: методы `ChargeProjectBalance` (map `ErrBalanceInsuffient→FailedPrecondition`,
  `ErrProjectNotFound→NotFound`, `ErrInvalidChargeAmount→InvalidArgument`) и `RefundProjectBalance`.

### 3.3. gateway (`ucode_go_admin_api_gateway`)

- **`api/api.go`**: новый роут `ugenTemplate.PATCH("/:id/price", h.V1.SetUgenTemplatePrice)` в группе `v1Admin`.
- **`api/handlers/v1/ugen_template.go`**:
  - `ugenTemplateResponse` + `newUgenTemplateResponse`: поля `price` / `currency_id`.
  - Хендлер **`SetUgenTemplatePrice`** (+ struct `setUgenTemplatePriceRequest{price, currency_id}`), валидация `price>=0`.
  - Create/Update: цены нет.
  - **`CreateProjectFromTemplate` переписан** (см. раздел 4).
  - Хелпер **`respondBillingError`** (маппинг gRPC-кодов → 402/404/400/500).
  - `CreateProjectFromTemplateReq` получил опциональный `idempotency_key`.

---

## 4. Как теперь работает `CreateProjectFromTemplate` (важно!)

Была большая чистка + фикс бага `project not found: <uuid>`. Правило источников данных:

- **MCP-проект (name / project_env / app_visibility / project_type)** → `GetMcpProjectFiles(ResourceEnvId =
  tmpl.source_resource_env_id, Id = tmpl.mcp_project_id, WithoutFiles: true)`. Файлы отсюда НЕ берём.
- **Микрофронт-файлы** → `getTemplateMicrofrontendFiles(...)` **напрямую из репозитория микрофронта**
  (HTTP в go-function-service по `repo_id`), ключ — `tmpl.source_mcp_resource_env_id`. НЕ из `project_files`.
- **Данные (таблицы/строки/меню/лейауты/вьюхи/события)** → `copyUgenTemplateData` из `tmpl.source_resource_env_id`.
- **Новый MCP-проект + AI-чат** создаются в builder-окружении **head-проекта** (`mainService` /
  `mainResourceEnvID = headResource.ResourceEnvironmentId`), как у обычных u-gen проектов.
- **Без фолбэков** — читаем строго `tmpl.GetSource*()`.

Флоу целиком: bind req → project_id/environment_id из контекста → authInfo → tmpl(GetById) →
headProject/headResource → mainService/mainResourceEnvID → sourceService(source_project_id, source_node_type) →
sourceMcp (метаданные) → sourceMcpFiles (микрофронт) → **charge (если price>0)** → `provision()` closure
(создать project/env/resource/service, apiKey, CreateMcpProject, CreateChat, copyUgenTemplateData, publish, UpdateMcpProject)
→ при ошибке **refund** → ответ (+ `charged_amount`/`charge_transaction_id` если платно).

Причина старого бага: MCP-проект искали в неправильном resource env (фолбэк на текущий проект).

---

## 5. Эндпоинты (итог)

| Метод | Путь | Кто | Назначение |
|-------|------|-----|-----------|
| GET | `/v1/ugen-template/public[/:id]` | все | Каталог (без auth) |
| GET | `/v1/ugen-template[/:id]` | auth | Каталог (+ current_user_reaction) |
| POST | `/v1/ugen-template` | юзер | Создать шаблон (без цены) |
| PUT | `/v1/ugen-template/:id` | юзер | Обновить (без цены) |
| **PATCH** | **`/v1/ugen-template/:id/price`** | **админ** | **Задать цену** |
| DELETE | `/v1/ugen-template/:id` | юзер | Удалить |
| POST/DELETE/GET | `/v1/ugen-template/:id/reaction[s]` | юзер | Лайк/дизлайк |
| POST | `/v1/ugen-template/create-project` | юзер | **Использовать шаблон (+оплата)** |

Ответы завёрнуты в `{status, description, data, custom_message}`. Коды: 200 OK, 400 bad, 401 unauth,
**402 PAYMENT_REQUIRED (нет средств)**, 404 not found, 500 grpc.

---

## 6. Документация для фронта (уже написана)

- `docs/ugen_template_pricing_frontend.md` — для юзер-приложения (create/use template + биллинг + что изменилось в create).
- `docs/ugen_template_pricing_admin.md` — для админки (установка цены).

---

## 7. ЧТО ОСТАЛОСЬ СДЕЛАТЬ (для пользователя)

1. **Регенерировать proto** в ОБЕИХ репах:
   `make copy-proto-module && make gen-proto-module` (в `ucode_go_admin_api_gateway` и `ucode_go_company_service`).
2. `go build ./...` в обеих репах. Сейчас IDE показывает «missing proto symbols» (`SetPrice`, `GetPrice`,
   `ChargeProjectBalance`, `RefundProjectBalance`, `SetUgenTemplatePriceReq`, `ChargeProjectBalanceRequest`) —
   **это ожидаемо**, исчезнет после регена.
3. Применить миграцию `87` в company_service.
4. (Опц.) Если нужен жёсткий guard «только супер-админ» на `PATCH /:id/price` — сейчас он в `v1Admin`
   (та же auth, что create/update), как у token packs. Механизма super-admin в gateway не нашли; спросить если надо.

---

## 8. Проверки, которые прошли

- `gofmt -l` — чисто на всех изменённых Go-файлах.
- `protoc` parse обоих proto — exit 0 (реген не упадёт).
- Ручной аудит: выравнивание SQL-колонок RETURNING/SELECT ↔ Scan во всех запросах; счётчики
  аргументов/плейсхолдеров; uuid→string и numeric→float сканы совпадают с рабочим `insertTransaction`.
- Найден и исправлен реальный баг (в Update передавались лишние `req.Price/req.CurrencyId` — 17 арг на 15 плейсхолдеров).

---

## 9. Память (auto-memory)

Записано в `MEMORY.md` → `project_paid_templates.md` (решения, источники данных create-project, пути доков).
Новый чат подтянет это автоматически.
