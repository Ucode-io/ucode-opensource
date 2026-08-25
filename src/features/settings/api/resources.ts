import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/shared/api/client";
import { useSession } from "@/shared/api/use-session";
import i18n from "@/shared/lib/i18n";
import { keys } from "@/shared/lib/query-keys";
import { reportError, toast } from "@/shared/lib/toast";

/**
 * Ресурсы проекта — чужие службы, которыми он пользуется.
 *
 * Отправка кодов подтверждения, репозиторий с кодом функций, панель
 * аналитики, перекодировщик видео. Всё это строки одной таблицы
 * (`project_resource`): имя, тип и мешок настроек под типом.
 *
 * Ресурс принадлежит ОКРУЖЕНИЮ, а не проекту: заведённый в dev в prod
 * не появится (`resource.go:1624`). Отсюда окружение в ключе кэша.
 *
 * Не путать с «Внешними базами» (api/connections.ts): там чужой postgres,
 * таблицы которого показываются как свои. Завести его можно и ресурсом
 * типа POSTGRESQL — ручка зовёт тот же `CreateConnectionAndSchema`
 * (`resource.go:2582`), — но без выбора таблиц это половина дела,
 * поэтому здесь такой тип не предлагается.
 */
const RESOURCES = "/v2/company/project/resource";

/** Переменные ресурса живут своей ручкой и своей версией API. */
const VARIABLES = "/v1/company/project/resource-variable";

/** Переподключение базы — ручка первой версии, соседняя с этими. */
const RECONNECT = "/v1/company/project/resource/reconnect";

/**
 * `toggle` — единственное поле-флажок на весь раздел: `ssl_mode`
 * у PostgreSQL. В proto это `bool` (`PostgresCredentials`, поле 7),
 * а не строка «true», поэтому у него своя ветка и в форме, и в теле
 * запроса. Черновик при этом остаётся строковым: у формы одно хранилище
 * на все типы.
 */
export type FieldKind = "text" | "password" | "number" | "toggle";

export type ResourceField = {
  /**
   * Имя поля в JSON настроек. Оно же ключ подписи (`resources.field.*`)
   * и пояснения (`resources.fieldHint.*`) — второго имени у поля нет.
   */
  key: string;
  kind: FieldKind;
  /** У поля есть пояснение под ним. */
  hint?: true;
};

/** Группы в выборе типа: по поводу, а не по вендору. */
export type ResourceGroup = "otp" | "code" | "bi" | "db" | "other";

export const RESOURCE_GROUPS: ResourceGroup[] = ["otp", "code", "bi", "db", "other"];

export type ResourceSpec = {
  /**
   * Номер типа в proto. При создании тип уезжает ЧИСЛОМ: шлюз читает
   * тело обычным encoding/json, а `type` там — enum, то есть int32.
   * Строка «SMS» в этом поле даёт 400 ещё до разбора (`project_resource.go:35`).
   */
  code: number;
  /** Ключ, под которым настройки этого типа лежат в `settings`. */
  settingsKey?: string;
  fields?: readonly ResourceField[];
  /**
   * Поля, которые спрашиваются ТОЛЬКО при заведении.
   *
   * Нужны базам: подключить свою просят полными реквизитами — с паролем
   * и режимом SSL, — а показываем мы у них потом лишь то, что отдаёт
   * список. Заведённая строка живёт в `project_resource`, системная
   * приезжает из `resource` (UNION в `resource.go:1626`), и различить
   * их в ответе нечем; поэтому правка выключена у обеих (см. readOnly).
   */
  createFields?: readonly ResourceField[];
  /** Учётные данные выдаёт сам бэкенд: показать можно, править нечем. */
  readOnly?: true;
  /**
   * Заводит и правит такой ресурс не этот раздел: у типа своя ручка
   * подключения (OAuth, вебхук) или он вовсе создаётся вместе с проектом.
   * Карточка такого ресурса открывается на чтение.
   */
  managed?: true;
  /**
   * Строка проекта, а не этой таблицы: база самого проекта. Удалить её
   * этой ручкой нельзя — `DELETE … RETURNING id` не найдёт строки
   * и ответит ошибкой.
   */
  system?: true;
  /** Отключается своей ручкой: эта на удаление отвечает отказом. */
  noDelete?: true;
  /**
   * Заводится НА ПЛАТФОРМЕ, а не подключается: реквизиты придумывает
   * бэкенд. Отдельная ручка — см. useProvisionResource.
   */
  provision?: true;
  /** Вместо настроек — список пар «ключ-значение». */
  variables?: true;
  /**
   * Базу можно переподключить: службы получат учётку из хранилища
   * заново. Только mongo и postgres — у остальных типов в ручке
   * нет ветки вовсе (`resource.go:1113`), и она молча ничего не делает.
   */
  reconnect?: true;
  /** Группа в выборе типа. Нет группы — тип не предлагается заводить. */
  group?: ResourceGroup;
};

/** Поля кода подтверждения одни и те же у всех трёх отправителей. */
const OTP_FIELDS = [
  { key: "number_of_otp", kind: "number", hint: true },
  { key: "default_otp", kind: "password", hint: true },
] as const satisfies readonly ResourceField[];

/**
 * Системные строки — база самого проекта. В списке они приезжают из
 * другой таблицы (`resource`, UNION в `resource.go:1626`), а поштучно
 * шлюз отдаёт их через запасной запрос и кладёт host/port/username/
 * database в `settings.postgres` независимо от того, mongo это или
 * clickhouse (`resource.go:1812`). Отсюда общий набор полей.
 */
const SYSTEM: ResourceSpec = {
  code: 0,
  settingsKey: "postgres",
  readOnly: true,
  managed: true,
  system: true,
  fields: [
    { key: "host", kind: "text" },
    { key: "port", kind: "text" },
    { key: "username", kind: "text" },
    { key: "database", kind: "text" },
  ],
};

export const RESOURCE_SPECS: Record<string, ResourceSpec> = {
  /* Коды подтверждения. Читается ПЕРВЫЙ ресурс типа и только он
     (`session_service_v2.go:1104`) — об этом говорит и форма. */
  SMS: {
    code: 6,
    settingsKey: "sms",
    group: "otp",
    fields: [
      { key: "login", kind: "text" },
      { key: "password", kind: "password" },
      { key: "originator", kind: "text" },
      ...OTP_FIELDS,
    ],
  },
  SMTP: {
    code: 7,
    settingsKey: "smtp",
    group: "otp",
    fields: [
      { key: "email", kind: "text" },
      { key: "password", kind: "password" },
      ...OTP_FIELDS,
    ],
  },
  MAILCHIMP: {
    code: 14,
    settingsKey: "mailchimp",
    group: "otp",
    fields: [
      { key: "api_key", kind: "password" },
      { key: "from_email", kind: "text" },
      ...OTP_FIELDS,
    ],
  },

  /* Репозитории. Токен вводится руками: OAuth-дверь старой админки
     требует своего приложения и своего адреса возврата, а хранит она
     в итоге ровно эти два поля (`resource_service.proto:575`). */
  GITHUB: {
    code: 5,
    settingsKey: "github",
    group: "code",
    fields: [
      { key: "username", kind: "text" },
      { key: "token", kind: "password" },
    ],
  },
  GITLAB: {
    code: 8,
    settingsKey: "gitlab",
    group: "code",
    fields: [
      { key: "username", kind: "text" },
      { key: "token", kind: "password" },
      { key: "base_url", kind: "text" },
    ],
  },

  /* Панели аналитики. Заводятся одним именем: учётку, пароль и адрес
     выдаёт бэкенд при создании (`resource.go:2546`), присланные —
     затирает. Поэтому только чтение. */
  SUPERSET: {
    code: 11,
    settingsKey: "superset",
    group: "bi",
    readOnly: true,
    fields: [
      { key: "url", kind: "text" },
      { key: "username", kind: "text" },
      { key: "password", kind: "password" },
    ],
  },
  METABASE: {
    code: 12,
    settingsKey: "metabase",
    group: "bi",
    readOnly: true,
    fields: [
      { key: "url", kind: "text" },
      { key: "username", kind: "text" },
      { key: "password", kind: "password" },
    ],
  },

  /* Перекодировщик: заводится именем, настроек не имеет вовсе —
     бэкенд заводит компанию и проект на своей стороне
     (`resource.go:2225`). */
  TRANSCODER: { code: 13, group: "other", readOnly: true },

  /* Свой набор пар «ключ-значение». Ими же живут функции: значение
     достают по имени (`GET /resource-variable/single?key=`). */
  REST: { code: 4, group: "other", variables: true },

  MONGODB: { ...SYSTEM, code: 1, reconnect: true },
  /*
   * ClickHouse не подключают, а ЗАВОДЯТ: реквизиты придумывает
   * платформа (`company_service/grpc/service/resource.go:577` — имя базы,
   * пользователь и пароль собираются там же). Поэтому у него группа
   * есть, а полей заведения нет — см. provisionable.
   */
  CLICKHOUSE: { ...SYSTEM, code: 2, group: "db", provision: true },
  POSTGRESQL: {
    ...SYSTEM,
    code: 3,
    reconnect: true,
    group: "db",
    createFields: [
      { key: "host", kind: "text" },
      { key: "port", kind: "text" },
      { key: "database", kind: "text" },
      { key: "username", kind: "text" },
      { key: "password", kind: "password" },
      { key: "connection_name", kind: "text" },
      { key: "ssl_mode", kind: "toggle" },
    ],
  },

  /* Подключаются своей дверью, и шлюз честно отказывает этой
     (`project_resource.go:56`, `:64`, `:75`). Показываем, но не трогаем. */
  GOOGLE_DRIVE: { code: 15, managed: true },
  GOOGLE_CALENDAR: { code: 16, managed: true },
  META_LEADS: { code: 18, managed: true },
  GOOGLE_LEADS: { code: 20, managed: true },

  /* Эти двое не дают и удалить: шлюз отсылает к своей ручке отключения
     (`project_resource.go:729`, `:733`). */
  TELEGRAM: { code: 17, managed: true, noDelete: true },
  INSTAGRAM: { code: 19, managed: true, noDelete: true },

  GIT: { code: 9, managed: true },
  BITBUCKET: { code: 10, managed: true },
  NOT_DECIDED: { code: 0, managed: true },
};

/**
 * Названия типов. Не переводятся намеренно: это имена служб, и «Гитхаб»
 * никому не помогает. Переведено то, что несёт смысл, — пояснения
 * в выборе типа (`resources.about.*`).
 */
export const RESOURCE_LABELS: Record<string, string> = {
  SMS: "SMS",
  SMTP: "SMTP",
  MAILCHIMP: "Mailchimp",
  GITHUB: "GitHub",
  GITLAB: "GitLab",
  SUPERSET: "Superset",
  METABASE: "Metabase",
  TRANSCODER: "Transcoder",
  REST: "REST",
  MONGODB: "MongoDB",
  CLICKHOUSE: "ClickHouse",
  POSTGRESQL: "PostgreSQL",
  GOOGLE_DRIVE: "Google Drive",
  GOOGLE_CALENDAR: "Google Calendar",
  TELEGRAM: "Telegram",
  META_LEADS: "Meta Leads",
  INSTAGRAM: "Instagram",
  GOOGLE_LEADS: "Google Leads",
  GIT: "Git",
  BITBUCKET: "Bitbucket",
  NOT_DECIDED: "—",
};

/** Что предлагается завести, по группам и в порядке групп. */
export const CREATABLE: { group: ResourceGroup; kinds: string[] }[] = RESOURCE_GROUPS.map(
  (group) => ({
    group,
    kinds: Object.keys(RESOURCE_SPECS).filter((kind) => RESOURCE_SPECS[kind]?.group === group),
  }),
).filter((entry) => entry.kinds.length > 0);

export type ResourceVariable = {
  /** Пусто — переменная ещё не сохранена. По этому её и заводят. */
  id: string;
  key: string;
  value: string;
};

export type Resource = {
  id: string;
  name: string;
  /** Имя типа из proto: SMS, GITHUB, … Оно же ключ в RESOURCE_SPECS. */
  kind: string;
  /** Поля настроек этого типа — плоско и строками, как их правит форма. */
  settings: Record<string, string>;
  variables: ResourceVariable[];
};

type ResourceDto = {
  id?: string;
  name?: string;
  type?: string;
  settings?: Record<string, Record<string, unknown> | undefined>;
  variables?: { id?: string; key?: string; value?: string }[];
};

/** Значение поля строкой: число приезжает числом, а правят его текстом. */
function text(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number") return String(value);
  return "";
}

export function toResource(dto: ResourceDto): Resource {
  const kind = dto.type?.trim() || "NOT_DECIDED";
  const spec = RESOURCE_SPECS[kind];
  const raw = spec?.settingsKey ? dto.settings?.[spec.settingsKey] : undefined;

  return {
    id: dto.id ?? "",
    name: dto.name?.trim() || RESOURCE_LABELS[kind] || kind,
    kind,
    settings: Object.fromEntries(
      (spec?.fields ?? []).map((field) => [field.key, text(raw?.[field.key])]),
    ),
    /*
     * Переменные приезжают из JSON_AGG по LEFT JOIN (`resource.go:1725`):
     * когда их нет, приходит не пустой список, а ОДНА строка из null-ов.
     * Отсюда отбор по id, а не по длине.
     */
    variables: (dto.variables ?? [])
      .filter((variable) => variable.id)
      .map((variable) => ({
        id: variable.id ?? "",
        key: variable.key ?? "",
        value: variable.value ?? "",
      })),
  };
}

/**
 * Настройки обратно в форму бэкенда.
 *
 * Числовые поля уезжают числами: `number_of_otp` в proto — int32,
 * и строка в нём валит разбор тела целиком, а не одно поле.
 *
 * Набор полный всегда: PUT кладёт `settings` целиком
 * (`resource.go:1850`), и отправить половину значит стереть вторую.
 */
export function toSettings(kind: string, values: Record<string, string>, creating = false) {
  const spec = RESOURCE_SPECS[kind];
  // При заведении набор бывает шире показанного: у баз спрашивают пароль
  // и имя подключения, которых в списке потом не видно.
  const fields = (creating && spec?.createFields) || spec?.fields;
  if (!spec?.settingsKey || !fields?.length) return undefined;

  const body: Record<string, string | number | boolean> = {};
  for (const field of fields) {
    const value = (values[field.key] ?? "").trim();

    // Флажок в proto — bool, и строка «true» в нём валит разбор тела
    // целиком, как и строка в числовом поле.
    if (field.kind === "toggle") body[field.key] = value === "true";
    else body[field.key] = field.kind === "number" ? Number(value) || 0 : value;
  }

  return { [spec.settingsKey]: body };
}

const NO_RESOURCES: Resource[] = [];

export function useResources() {
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.resources(projectId, envId),
    queryFn: () =>
      api.get<{ resources?: ResourceDto[] }>(RESOURCES, {
        params: { "project-id": projectId },
      }),
    enabled: Boolean(projectId && envId),
    staleTime: 60_000,
    select: (data): Resource[] => (data.resources ?? []).filter((dto) => dto.id).map(toResource),
  });

  return { resources: query.data ?? NO_RESOURCES, isLoading: query.isLoading };
}

/**
 * Один ресурс целиком. Форма правки читает ЕГО, а не строку списка:
 * переменных в списке нет вовсе, а у системных строк там нет и настроек
 * (`resource.go:1634` отдаёт по ним NULL).
 */
export function useResource(id: string) {
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: keys.settings.resource(projectId, envId, id),
    queryFn: () =>
      api.get<ResourceDto>(`${RESOURCES}/${id}`, { params: { "project-id": projectId } }),
    enabled: Boolean(projectId && envId && id),
    select: toResource,
  });

  return { resource: query.data ?? null, isLoading: query.isLoading };
}

export type ResourceDraft = {
  name: string;
  settings: Record<string, string>;
  variables: ResourceVariable[];
};

/** Пустой черновик под тип: поля те, что у типа есть. */
export function emptyDraft(kind: string): ResourceDraft {
  const spec = RESOURCE_SPECS[kind];

  return {
    name: "",
    settings: Object.fromEntries((spec?.fields ?? []).map((field) => [field.key, ""])),
    variables: [],
  };
}

export function toDraft(resource: Resource): ResourceDraft {
  return {
    name: resource.name,
    settings: { ...resource.settings },
    variables: resource.variables.map((variable) => ({ ...variable })),
  };
}

export function useCreateResource() {
  const invalidate = useInvalidateResources();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: ({ kind, draft }: { kind: string; draft: ResourceDraft }) => {
      /* У типов с выданной учёткой настроек в форме нет — и посылать
         пустые поля значит отправить то, чего человек не вводил.
         Бэкенд их всё равно перезапишет своими. У баз исключение:
         их подключают своими реквизитами, и спрашиваются они только
         здесь (см. createFields). */
      const spec = RESOURCE_SPECS[kind];
      const settings =
        spec?.readOnly && !spec.createFields
          ? undefined
          : toSettings(kind, draft.settings, true);
      const variables = toVariables(draft.variables);

      return api.post<unknown>(
        RESOURCES,
        {
          name: draft.name.trim(),
          type: spec?.code ?? 0,
          ...(settings ? { settings } : {}),
          /* При создании переменные заводит сама ручка ресурса
             (`resource.go:2599`) — отдельного запроса не нужно. */
          ...(variables.length ? { variables } : {}),
        },
        { params: { "project-id": projectId } },
      );
    },
    onError: (error) => reportError(error, "common.createFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("resources.created"));
      await invalidate();
    },
  });
}

/**
 * Правка ресурса.
 *
 * Тип не отправляется: UPDATE его не трогает (`resource.go:1848` пишет
 * имя, настройки и external_id), а прислать его значит соврать форме,
 * будто тип можно сменить.
 *
 * Настройки уезжают ВСЕГДА и целиком — даже у типов, где их не правят:
 * они выданы бэкендом, и переименование без них обнулило бы учётку.
 */
export function useUpdateResource() {
  const invalidate = useInvalidateResources();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: async ({ resource, draft }: { resource: Resource; draft: ResourceDraft }) => {
      const settings = toSettings(resource.kind, draft.settings);

      await api.put<unknown>(
        RESOURCES,
        {
          id: resource.id,
          name: draft.name.trim(),
          ...(settings ? { settings } : {}),
        },
        { params: { "project-id": projectId } },
      );

      if (!RESOURCE_SPECS[resource.kind]?.variables) return;

      /*
       * Переменные — три отдельных действия одной ручкой и ещё одной.
       * PUT принимает список и различает создание и правку по пустому id
       * (`resource.go:2147`), а удалять умеет только DELETE по одной.
       */
      const kept = toVariables(draft.variables);

      if (kept.length) {
        await api.put<unknown>(
          VARIABLES,
          { project_resource_id: resource.id, variables: kept },
          { params: { "project-id": projectId } },
        );
      }

      const removed = resource.variables.filter(
        (old) => !draft.variables.some((variable) => variable.id === old.id),
      );

      await Promise.all(
        removed.map((variable) =>
          api.delete<unknown>(`${VARIABLES}/${variable.id}`, {
            params: { "project-id": projectId },
          }),
        ),
      );
    },
    onError: (error) => reportError(error, "common.saveFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("settings.saved"));
      await invalidate();
    },
  });
}

export function useDeleteResource() {
  const invalidate = useInvalidateResources();
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) =>
      api.delete<unknown>(`${RESOURCES}/${id}`, { params: { "project-id": projectId } }),
    onError: (error) => reportError(error, "common.deleteFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("resources.deleted"));
      await invalidate();
    },
  });
}

/**
 * Службы, которым ручка переподключения раздаёт учётку базы. Числа —
 * это `ServiceType` из proto (`company.proto:8`), и в ответе они
 * приезжают числами: тело шлюз собирает обычным encoding/json.
 */
const SERVICE_NAMES: Record<number, string> = {
  1: "Builder",
  2: "Analytics",
  3: "Template",
  4: "Query",
  5: "Functions",
  6: "Web page",
  7: "API reference",
  8: "Postgres builder",
};

/**
 * Переподключить базу проекта: службы получают её учётку из хранилища
 * заново. Нужно, когда база сменила адрес или пароль, а службы остались
 * со старым.
 *
 * Ответ разбирается по каждой службе отдельно, и «не ответила» — это
 * ОТСУТСТВИЕ `status`, а не `false`: поле помечено omitempty, и ложь
 * из ответа выпадает (`projects_service.proto:374`). Старая админка
 * ответ не читала вовсе и писала «успешно» в любом случае.
 */
/**
 * Завести базу НА ПЛАТФОРМЕ, а не подключить свою.
 *
 * Ручка первой версии и с другим телом: `POST
 * /v1/company/project/create-resource`. Реквизитов она не принимает —
 * имя базы, пользователя и пароль собирает сама
 * (`company_service/grpc/service/resource.go:610–640`), а `node_type`
 * решает, на каком кластере заводить.
 *
 * Тело повторяет старую админку дословно (`Resources/Detail/index.jsx:337`):
 * `node_type: "LOW"` и `resource` с тем же типом и заголовком «Light».
 * Второго тарифа она не предлагала, и придумывать его нам неоткуда:
 * набор допустимых `node_type` живёт в конфигурации платформы.
 *
 * **В старой админке форма реквизитов для ClickHouse существует
 * и не используется.** `ClickHouseForm.jsx` собирает host, port,
 * username, password и database, а ветка отправки для типа 2 строит
 * тело заново и введённое не читает — значения просто теряются.
 * Поэтому у нас этой формы нет: заведение спрашивает только имя.
 */
export function useProvisionResource() {
  const invalidate = useInvalidateResources();
  const session = useSession();

  return useMutation({
    mutationFn: ({ kind, name }: { kind: string; name: string }) =>
      api.post<unknown>("/v1/company/project/create-resource", {
        name: name.trim(),
        project_id: session.getProjectId() ?? "",
        environment_id: session.getEnvironmentId() ?? "",
        node_type: NODE_TYPE,
        resource: {
          resource_type: RESOURCE_SPECS[kind]?.code ?? 0,
          node_type: NODE_TYPE,
          title: NODE_TITLE,
          is_configured: true,
        },
      }),
    onError: (error) => reportError(error, "resources.provisionFailed"),
    onSuccess: async () => {
      toast.success(i18n.t("resources.provisioned"));
      await invalidate();
    },
  });
}

/** Тариф кластера. Единственный, который знает старая админка. */
const NODE_TYPE = "LOW";
const NODE_TITLE = "Light";

/**
 * В каких окружениях ресурс подключён.
 *
 * Ответ — строки `resource_environment`: у ресурса своя строка на
 * окружение, и «подключено» это её `is_configured`. Старая админка
 * показывает тот же список сбоку формы (`ResourceEnvironment.jsx`),
 * но выбрать в нём ничего нельзя — обработчик клика там закомментирован.
 * У нас он тоже только показывает: назначить окружение этой ручкой
 * нечем, она читающая.
 */
export function useResourceEnvironments(id: string) {
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  const query = useQuery({
    queryKey: [...keys.settings.resource(projectId, envId, id), "environments"] as const,
    queryFn: () =>
      api.get<{ resource_environments?: ResourceEnvironmentDto[] | null }>(
        `/v1/company/project/resource-environment/${id}`,
      ),
    enabled: Boolean(id),
  });

  return {
    environments: (query.data?.resource_environments ?? []).map((dto) => ({
      id: dto.environment_id ?? "",
      configured: dto.is_configured === true,
    })),
    isLoading: query.isLoading,
  };
}

type ResourceEnvironmentDto = { environment_id?: string; is_configured?: boolean };

export function useReconnectResource() {
  const projectId = useSession().getProjectId() ?? "";

  return useMutation({
    mutationFn: (id: string) =>
      api.post<{ reconnect_status?: { service_type?: number; status?: boolean }[] }>(
        RECONNECT,
        { id, project_id: projectId },
        { params: { "project-id": projectId } },
      ),
    onError: (error) => reportError(error, "resources.reconnectFailed"),
    onSuccess: (data) => {
      const statuses = data.reconnect_status ?? [];

      if (!statuses.length) {
        toast.error(i18n.t("resources.reconnectEmpty"));
        return;
      }

      const failed = statuses
        .filter((entry) => entry.status !== true)
        .map((entry) => SERVICE_NAMES[entry.service_type ?? 0] ?? String(entry.service_type ?? "?"));

      if (failed.length) {
        toast.error(i18n.t("resources.reconnectPartly", { services: failed.join(", ") }));
        return;
      }

      /* Подстановка называется total, а не count: `count` включает
         машинерию множественного числа i18next, а формы у нас одна. */
      toast.success(i18n.t("resources.reconnected", { total: statuses.length }));
    },
  });
}

/**
 * Строки формы → переменные для бэкенда. Без имени строка не уезжает
 * вовсе: это пустое поле формы, а не данные. Пустой `id` не отправляем —
 * по нему ручка и отличает новую переменную от правки.
 */
export function toVariables(variables: ResourceVariable[]) {
  return variables
    .filter((variable) => variable.key.trim())
    .map((variable) => ({
      ...(variable.id ? { id: variable.id } : {}),
      key: variable.key.trim(),
      value: variable.value,
    }));
}

function useInvalidateResources() {
  const queryClient = useQueryClient();
  const session = useSession();
  const projectId = session.getProjectId() ?? "";
  const envId = session.getEnvironmentId() ?? "";

  /* Ключ одного ресурса вложен в ключ списка — протухают оба разом. */
  return () =>
    queryClient.invalidateQueries({ queryKey: keys.settings.resources(projectId, envId) });
}
