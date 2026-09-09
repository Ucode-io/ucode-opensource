# Handoff — Per-user pricing для проектов из шаблона

> Полный контекст фичи «платный per-user seat». Продолжение работы по платным шаблонам
> (`docs/ugen_template_pricing_handoff.md`). Дата: 2026-07-02.

---

## 0. TL;DR

Шаблон может нести **цену за юзера** (`per_user_price`) вдобавок к цене импорта (`price`).
- Админ задаёт обе цены на шаблоне одним эндпоинтом `PATCH /v1/ugen-template/:id/price`.
- При импорте (`create-project`) `template.per_user_price` **копируется в проект** (`project.per_user_price` + валюта = `template.currency_id`, одна валюта на весь шаблон).
- При **добавлении юзера** в такой проект (`POST /v2/user` и `POST /v2/user/invite` в auth_service):
  - если `project.per_user_price > 0` → **тарифный лимит игнорируется**, цена списывается с **head (ugen) проекта компании** (`ChargeProjectBalance`), при провале — refund;
  - если `= 0` (обычные проекты) → всё как раньше (`checkUserProjectLimit` / `checkUgenBuildersLimit`).
- Нет средств → gRPC `FailedPrecondition` → **HTTP 402** (`code: "insufficient_balance"`).

Переиспользованы `ChargeProjectBalance` / `RefundProjectBalance` из фичи платных шаблонов — новый биллинг не писался.

Статус: **код готов**, ждёт регенерации proto (см. §7) и миграции 88.

---

## 1. Принятые продуктовые решения (подтверждены пользователем 2026-07-02)

1. **Только для проектов из шаблона.** Переключатель — сам факт `project.per_user_price > 0`. Обычные проекты не затронуты.
2. **Плательщик** — head (ugen) проект компании (там же, где деньги за импорт; целевой проект стартует с 0).
3. **Цена — свойство шаблона**, задаёт только админ; наследуется проектом при импорте (dynamic per project через копию в `project`).
4. **Одна валюта** — `template.currency_id` используется и для цены импорта, и для per-user (нет отдельного per_user_currency на шаблоне; на проекте есть `per_user_currency_id` = копия).
5. **Платны оба действия**: создание нового юзера (`V2CreateUser`) и приглашение существующего (`AddUserToProject`) — но только при создании **нового** membership (дубликаты не заряжаются).
6. **Refund on failure**, best-effort с loud-логом, detached-контекст.
7. Идемпотентность заряда — не нужна: повторный запрос ловится проверкой «уже участник» (→ ErrUserExists/AlreadyExists, без заряда); каждый external_id = свежий uuid, поэтому remove+re-add заряжается заново (это правильно).
8. **Первый юзер проекта — бесплатный** (fix 2026-07-02): owner создаётся фронтом ОТДЕЛЬНЫМ `POST /v2/user` уже после импорта (gateway create-project юзера не создаёт), когда цена уже на проекте — поэтому «отложить установку цены» не решает. Решение в `reserveUserSeat`: если `GetProjectUsersCount(project_id) == 0` → `(nil,nil)` (бесплатно, без лимита). Плата со 2-го юзера. Импорт `price=10$`,`per_user=2$` теперь спишет ровно 10$, не 12$.

---

## 2. Изменения по файлам

### 2.1 Proto (`ucode_protos` submodule, каноничная правка в checkout gateway)
- `ugen_template.proto`: `UgenTemplate.per_user_price = 24`; `SetUgenTemplatePriceReq.per_user_price = 4`.
- `projects_service.proto`:
  - `Project.per_user_price = 26`, `Project.per_user_currency_id = 27`;
  - `CreateProjectRequest.per_user_price = 7`, `per_user_currency_id = 8`;
  - новый RPC `GetUgenProjectByCompanyId(GetUgenProjectByCompanyIdReq{company_id}) returns (Project)`.
- `billing_service.proto`: **без изменений** (Charge/Refund уже есть с прошлой фичи).

### 2.2 company_service
- Миграция **`88_add_project_per_user_pricing.{up,down}.sql`**: `ugen_template.per_user_price`; `project.per_user_price` + `per_user_currency_id`; enum `user_seat_purchase` / `user_seat_refund`.
- `config/constants.go`: `TransactionTypeUserSeat` / `TransactionTypeUserSeatRfnd`.
- `storage/postgres/ugen_template.go`: `per_user_price` во всех scan'ах (Create/GetById/GetList/Update/SetPrice); `SetPrice` пишет `per_user_price = $4`.
- `storage/postgres/project.go`: Create пишет per-user колонки (`NULLIF($8,'')::uuid`); GetById читает их; новый метод **`GetUgenProjectByCompanyId`**. Update **не трогает** per_user_price (partial update — безопасно).
- `storage/postgres/ugen_template_billing.go`: `RefundProjectBalance` теперь маппит тип рефанда по типу заряда (template→template_refund, user_seat→user_seat_refund); guard расширен на оба.
- `storage/repo/project.go`: `GetUgenProjectByCompanyId` в интерфейсе.
- `grpc/service/project.go`: метод `GetUgenProjectByCompanyId` (пустой company_id → InvalidArgument; не найден → NotFound).

### 2.3 gateway
- `api/handlers/v1/ugen_template.go`:
  - `setUgenTemplatePriceRequest` + handler `SetUgenTemplatePrice`: поле `per_user_price` (валидация `>= 0`).
  - `ugenTemplateResponse` + `newUgenTemplateResponse`: `per_user_price`.
  - `CreateProjectFromTemplate`: в `CreateProjectRequest` копируются `PerUserPrice = tmpl.GetPerUserPrice()`, `PerUserCurrencyId = tmpl.GetCurrencyId()`.
- Роут `PATCH /:id/price` уже существует (v1Admin) — новый не нужен.

### 2.4 auth_service (сердце)
- **`grpc/service/user_seat_billing.go`** (новый): `reserveUserSeat` (резолв head-проекта → `ChargeProjectBalance`, `user_seat_purchase`, свежий external_id) и `releaseUserSeat` (best-effort `RefundProjectBalance`, detached ctx). Free-проект → `(nil, nil)`.
- `grpc/service/user_service_v2.go`:
  - `V2CreateUser`: сигнатура → named returns `(resp, err)`; priced-ветка (pre-check дубликата → charge → `defer` refund при `err && !seatCommitted`) vs free-ветка (старые лимиты); `seatCommitted = true` после успешного создания юзера, **до** отправки email (чтобы падение email не рефандило занятый seat).
  - `AddUserToProject`: priced-ветка (pre-check дубликата → charge → AddUserToProject → refund при ошибке) vs free-ветка (старый лимит).
- `api/handlers/handler.go`: `handleError` маппит `codes.FailedPrecondition` → **402** (`insufficient_balance`).
- `api/handlers/user_v2.go`: `AddUserToProject` HTTP теперь отдаёт ошибку через `handleError` (чтобы 402 сработал); `V2CreateUser` HTTP уже шёл через `handleError`.

---

## 3. Поток целиком

```
Админ: PATCH /v1/ugen-template/:id/price {price, currency_id, per_user_price}
Юзер:  POST /v1/ugen-template/create-project
         → charge импорта (если price>0)
         → Project.Create(per_user_price=tmpl.per_user_price, per_user_currency_id=tmpl.currency_id)
Юзер:  POST /v2/user | /v2/user/invite  (auth_service)
         → GetById(project); per_user_price>0 ?
             да  → pre-check дубликата → GetUgenProjectByCompanyId → ChargeProjectBalance(head) → создать membership → (fail ⇒ refund)
             нет → checkUserProjectLimit / checkUgenBuildersLimit (как раньше)
         → нет средств ⇒ 402 insufficient_balance
```

---

## 4. Известные границы (осознанно, не баги)

- **Self-registration (`RegisterUser`, /register) не заряжается** — это self-signup конечных юзеров, отдельный флоу. Если нужно заряжать и его — сказать, добавлю по тому же helper'у.
- **Object-builder-запись юзера в HTTP-хендлере `AddUserToProject`** идёт ПОСЛЕ gRPC-заряда/membership. Если она упадёт, seat уже заряжен (membership = источник истины; так же ведёт себя текущий код, который не откатывает membership). Refund покрывает только провал самого membership.

---

## 5. ЧТО ОСТАЛОСЬ (пользователю)

1. **Proto**: закоммитить+запушить submodule `ucode_protos` (правки в checkout gateway); в company_service **и** auth_service обновить указатель submodule (auth сейчас на старом коммите `f7fbeac`, gateway/company — `65adc8a`) → регенерить в **трёх** репах.
   - Пока не регенерено, IDE/сборка покажет отсутствующие символы (`PerUserPrice`, `GetUgenProjectByCompanyId`, `ChargeProjectBalanceRequest` в auth) — **это ожидаемо**.
2. `go build ./...` в трёх репах.
3. Применить миграцию `88` в company_service.
4. (Опц.) решить про self-registration (§4).

---

## 6. Проверки, пройденные локально
- `gofmt -l` — чисто на всех изменённых Go-файлах.
- Ручной аудит: счётчики колонок/плейсхолдеров/Scan во всех SQL (ugen_template ×5, project Create/GetById, GetUgenProjectByCompanyId); named-return + shadowing в V2CreateUser (поведение сохранено, defer видит финальный err); seatCommitted-флаг исключает refund при падении email.