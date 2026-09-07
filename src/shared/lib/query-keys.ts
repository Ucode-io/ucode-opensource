/**
 * Ключи кэша — только отсюда. В старом ucode одни и те же строки таблицы
 * лежали под четырьмя именами (GET_OBJECT_LIST, GET_OBJECTS_LIST,
 * GET_OBJECT_LIST_ALL, GET_OBJECTS_LIST_WITH_RELATIONS) — четыре копии
 * одних данных в памяти и четыре запроса вместо одного.
 *
 * Опечатка в строковом ключе создаёт второй кэш молча. Опечатка здесь —
 * ошибка компиляции.
 */
export const keys = {
  tables: {
    all: ["tables"] as const,
    list: (projectId: string) => [...keys.tables.all, projectId] as const,
    detail: (tableSlug: string) => [...keys.tables.all, "detail", tableSlug] as const,
    /** Схема: поля и связи. Два запроса, потому что бэкенд отдаёт их порознь. */
    fields: (tableSlug: string) => [...keys.tables.all, "fields", tableSlug] as const,
    relations: (tableSlug: string) => [...keys.tables.all, "relations", tableSlug] as const,
    /**
     * Подробности таблицы: поля с флагом `is_search` и view с правами
     * роли. Отдельный запрос, потому что и то и другое отдаёт только
     * POST /v1/table-details — ни в GET /v2/fields, ни в списке view
     * их нет вовсе.
     */
    details: (tableSlug: string) => [...keys.tables.all, "details", tableSlug] as const,
    /**
     * Строки чужой таблицы для выбора руками (условие агрегата).
     * Поиск в ключе: это и есть запрос.
     */
    relationRows: (tableSlug: string, search: string, limit: number) =>
      [...keys.tables.all, "relation-rows", tableSlug, search, limit] as const,
    /** Настройки одной связи: поля показа лежат только в ней. */
    relation: (tableSlug: string, relationId: string) =>
      [...keys.tables.all, "relations", tableSlug, relationId] as const,
  },
  workspace: {
    all: ["workspace"] as const,
    companies: (ownerId: string) => [...keys.workspace.all, "companies", ownerId] as const,
    projects: (companyId: string) => [...keys.workspace.all, "projects", companyId] as const,
    environments: (projectId: string) =>
      [...keys.workspace.all, "environments", projectId] as const,
    /**
     * Карточка проекта целиком: имя, логотип, языки данных, пояс,
     * валюта. Ключ ОДИН на всех, кто её читает, — и языки в сайдбаре,
     * и настройки проекта, и логотип в шапке берут один и тот же ответ.
     * Раньше тот же адрес лежал под двумя ключами, то есть грузился
     * дважды.
     */
    project: (projectId: string) => [...keys.workspace.all, "project", projectId] as const,
  },
  /**
   * Настройки: профиль человека и его сессии, настройки проекта
   * и справочники (языки, часовые пояса). Профиль живёт на сервере
   * авторизации, проект — на шлюзе; ключ один, потому что показывают
   * их в одном окне.
   */
  settings: {
    all: ["settings"] as const,
    profile: (userId: string) => [...keys.settings.all, "profile", userId] as const,
    sessions: (userId: string) => [...keys.settings.all, "sessions", userId] as const,
    /** Роли проекта: их список и права каждой на таблицы. */
    roles: (projectId: string) => [...keys.settings.all, "roles", projectId] as const,
    rolePermissions: (projectId: string, roleId: string) =>
      [...keys.settings.all, "roles", projectId, roleId] as const,
    /** Типы клиентов проекта: у роли обязательно есть один. */
    clientTypes: (projectId: string) => [...keys.settings.all, "client-types", projectId] as const,
    /**
     * Один тип целиком — его читает форма правки. Ключ вложен в список
     * намеренно: правка протухает список, а вместе с ним и карточку.
     */
    clientType: (projectId: string, id: string) =>
      [...keys.settings.clientTypes(projectId), id] as const,
    /** Таблицы входа проекта — из чего выбирает форма типа клиента. */
    loginTables: (projectId: string) => [...keys.settings.all, "login-tables", projectId] as const,
    /**
     * Таблицы аудитории: что приложение заказчика показывает этому типу
     * клиента. Не путать с `connections` ниже — там внешние базы.
     */
    clientConnections: (projectId: string, clientTypeId: string) =>
      [...keys.settings.all, "client-connections", projectId, clientTypeId] as const,
    /**
     * Права роли на пункты меню — по уровню дерева: бэкенд отдаёт их
     * по одному родителю, как и само меню.
     */
    menuPermissionsAll: () => [...keys.settings.all, "menu-permissions"] as const,
    menuPermissions: (projectId: string, roleId: string, parentId: string) =>
      [...keys.settings.menuPermissionsAll(), projectId, roleId, parentId] as const,
    /**
     * Свои права роли — по уровню дерева, как и права на меню. В ключе
     * ещё и тип клиента: право принадлежит аудитории, а не только роли.
     */
    customPermissionsAll: () => [...keys.settings.all, "custom-permissions"] as const,
    customPermissions: (roleId: string, clientTypeId: string, parentId: string) =>
      [...keys.settings.customPermissionsAll(), roleId, clientTypeId, parentId] as const,
    /** Наборы значков iconify. Общие на всё приложение, а не на проект. */
    iconCollections: () => [...keys.settings.all, "icon-collections"] as const,
    /** Справочник: LANGUAGE, TIMEZONE, CURRENCY. Общий на проект. */
    options: (projectId: string, type: string) =>
      [...keys.settings.all, "options", projectId, type] as const,
    /**
     * Люди проекта. Список приходит по ОДНОМУ типу клиента — так его
     * отдаёт ручка, — поэтому тип в ключе. Поиск и страница тоже:
     * это и есть запрос, а не отбор в памяти.
     */
    usersAll: () => [...keys.settings.all, "users"] as const,
    users: (projectId: string, clientTypeId: string, search: string, page: number) =>
      [...keys.settings.usersAll(), projectId, clientTypeId, search, page] as const,
    /**
     * API-ключи. Ключ выдаётся в ОДНОМ окружении (`environment_id`
     * берётся из заголовка запроса), поэтому окружение в ключе кэша:
     * иначе список prod и dev делил бы одну ячейку.
     */
    apiKeysAll: () => [...keys.settings.all, "api-keys"] as const,
    apiKeys: (projectId: string, envId: string, search: string, page: number) =>
      [...keys.settings.apiKeysAll(), projectId, envId, search, page] as const,
    /** Справочник платформ клиента — общий на всё приложение. */
    clientPlatforms: () => [...keys.settings.all, "client-platforms"] as const,
    /**
     * Ресурсы проекта: чужие службы, которыми он пользуется. Окружение
     * в ключе — строка ресурса заводится в ОДНОМ окружении (`project_id`
     * и `environment_id` в первичном отборе, `resource.go:1624`), и списки
     * prod и dev делить одну ячейку не должны.
     */
    resources: (projectId: string, envId: string) =>
      [...keys.settings.all, "resources", projectId, envId] as const,
    /**
     * Один ресурс целиком: его читает форма правки. Только этот ответ
     * несёт переменные и настройки системных строк — в списке их нет.
     */
    resource: (projectId: string, envId: string, id: string) =>
      [...keys.settings.resources(projectId, envId), id] as const,
    /**
     * Дашборды Metabase. Ключ — логин, а не ресурс: список приходит
     * из самого Metabase, и у другой учётки он другой, даже если строка
     * ресурса та же. Пароля в ключе нет намеренно: учётку выдаёт бэкенд
     * (METABASE — readOnly), в форме её не правят, и в паре меняется
     * всегда логин.
     */
    metabaseDashboards: (username: string) =>
      [...keys.settings.all, "metabase-dashboards", username] as const,
    /**
     * Подключённый аккаунт репозитория. Окружение в ключе: интеграция
     * заводится в паре «проект + окружение», как и ресурсы.
     */
    integration: (envId: string, provider: string) =>
      [...keys.settings.all, "integration", envId, provider] as const,
    /**
     * Журнал изменений: своя запись у каждого окружения. Отбор входит
     * в ключ целиком — фильтруется на сервере, а не у нас.
     */
    activityAll: () => [...keys.settings.all, "activity"] as const,
    activity: (envId: string, filters: Record<string, string | number>) =>
      [...keys.settings.activityAll(), envId, filters] as const,
    activityEntry: (envId: string, id: string) =>
      [...keys.settings.activityAll(), envId, "entry", id] as const,
    /** Расход API-запросов: лимит месяца и разбивка по маршрутам. На проект. */
    usage: (projectId: string, params: Record<string, string | number>) =>
      [...keys.settings.all, "usage", projectId, params] as const,
    /** Отправители одного маршрута — фильтр маршрута входит в params. */
    usageActors: (projectId: string, params: Record<string, string | number>) =>
      [...keys.settings.all, "usage", projectId, "actors", params] as const,
    /** Справочник «id пользователя → имя» для подписей отправителей. */
    usageSenderNames: (projectId: string) =>
      [...keys.settings.all, "usage", projectId, "senderNames"] as const,

    usageTimeline: (projectId: string, params: Record<string, string | number>) =>
      [...keys.settings.all, "usage", projectId, "timeline", params] as const,
    /** Свои эндпоинты (`/x-api/...`): список на проект и окружение. */
    endpoints: (projectId: string, envId: string) =>
      [...keys.settings.all, "endpoints", projectId, envId] as const,
    /**
     * Внешние базы: подключения проекта и таблицы одного подключения.
     * Окружение в ключе — подключение заводится в его ресурсе.
     */
    connections: (envId: string) => [...keys.settings.all, "connections", envId] as const,
    connectionTables: (envId: string, connectionId: string) =>
      [...keys.settings.connections(envId), connectionId] as const,
  },
  icons: {
    all: ["icons"] as const,
    search: (query: string) => [...keys.icons.all, query] as const,
  },
  /**
   * Действия таблицы (automation): что можно запустить над отмеченными
   * строками. Живут при таблице, поэтому и ключ по слагу.
   */
  actions: {
    all: ["actions"] as const,
    byTable: (tableSlug: string) => [...keys.actions.all, tableSlug] as const,
  },
  /**
   * Файловое хранилище проекта. Папка — часть ключа: пункт меню и есть
   * папка, и списки соседних папок друг друга не трогают.
   */
  files: {
    all: ["files"] as const,
    folder: (folder: string) => [...keys.files.all, folder] as const,
    list: (folder: string, search: string) => [...keys.files.folder(folder), search] as const,
  },
  /** Шаблоны проекта: готовые наборы таблиц. Список один на проект. */
  templates: {
    all: ["templates"] as const,
    list: () => [...keys.templates.all] as const,
  },
  /**
   * Шаблоны ДОКУМЕНТОВ: печатные формы записи, свои у каждой таблицы.
   * Не путать с `templates` выше — там наборы таблиц целого проекта.
   */
  docs: {
    all: ["docs"] as const,
    templates: (tableSlug: string) => [...keys.docs.all, tableSlug] as const,
    /** HTML-шаблоны той же таблицы: другое хранилище и другая ручка. */
    htmlTemplates: (tableSlug: string) => [...keys.docs.all, "html", tableSlug] as const,
  },
  /**
   * Микрофронтенды: по одному запросу на пункт меню. В ключе окружение —
   * адрес сборки у prod и dev разный.
   */
  microfrontends: {
    all: ["microfrontends"] as const,
    byId: (envId: string, id: string) => [...keys.microfrontends.all, envId, id] as const,
    /**
     * Версии, снимок версии и «есть ли что публиковать» — всё по номеру
     * репозитория в GitLab, а не по нашему id: этими ручками заведует
     * GitLab, и ключ у них там.
     */
    commits: (repoId: string) => [...keys.microfrontends.all, "commits", repoId] as const,
    filesAt: (repoId: string, sha: string) =>
      [...keys.microfrontends.all, "files-at", repoId, sha] as const,
    promoteChanges: (repoId: string) =>
      [...keys.microfrontends.all, "promote-changes", repoId] as const,
    pipeline: (repoId: string, pipelineId: string) =>
      [...keys.microfrontends.all, "pipeline", repoId, pipelineId] as const,
    /** Подмена экрана входа: ключ — поддомен, к нему и привязка. */
    loginBinding: (subdomain: string) =>
      [...keys.microfrontends.all, "login", subdomain] as const,
  },
  /**
   * Помощник: список прошлых бесед проекта. Сама переписка в кэше
   * не живёт — она пишется потоком и хранится в состоянии панели.
   */
  copilot: {
    all: ["copilot"] as const,
    chats: (projectId: string) => [...keys.copilot.all, "chats", projectId] as const,
  },
  /** Функции проекта: их зовут поля-кнопки. Список один на окружение. */
  functions: {
    all: ["functions"] as const,
    list: (envId: string) => [...keys.functions.all, envId] as const,
    /** Страница раздела функций: у него свой поиск и свои страницы. */
    page: (envId: string, search: string, page: number) =>
      [...keys.functions.all, envId, "page", search, page] as const,
    /**
     * Исходники функции. Ответ — весь репозиторий разом, поэтому
     * запрашивается он только у открытой карточки.
     */
    codebase: (envId: string, id: string) =>
      [...keys.functions.all, envId, "codebase", id] as const,
    /** Журнал выполнения функций: отбор входит в ключ целиком. */
    logs: (envId: string, filters: Record<string, string | number>) =>
      [...keys.functions.all, envId, "logs", filters] as const,
  },
  /**
   * В ключ входит и окружение: меню в prod и dev разное, и без него
   * данные двух окружений делили бы одну ячейку кэша. Заодно это и есть
   * то, что перезапрашивает сайдбар после переключения — меняется ключ.
   */
  menus: {
    all: ["menus"] as const,
    /** Дети одного уровня: бэкенд отдаёт меню только по parent_id. */
    children: (projectId: string, envId: string, parentId: string) =>
      [...keys.menus.all, projectId, envId, "children", parentId] as const,
    detail: (projectId: string, envId: string, menuId: string) =>
      [...keys.menus.all, projectId, envId, "detail", menuId] as const,
    /**
     * Всё дерево целиком — его собирает поиск, обходя уровни. Набранного
     * в ключе нет: дерево читается один раз, а отбор идёт по нему в памяти.
     */
    tree: (projectId: string, envId: string) =>
      [...keys.menus.all, projectId, envId, "tree"] as const,
  },
  views: {
    all: ["views"] as const,
    /**
     * Набор view принадлежит пункту меню, а не таблице: два пункта могут
     * показывать одну таблицу разными наборами. Окружение в ключе по той
     * же причине, что и у меню — настройки view в prod и dev разные.
     */
    byMenu: (envId: string, menuId: string) => [...keys.views.all, envId, "menu", menuId] as const,
    detail: (tableSlug: string, viewId: string) =>
      [...keys.views.all, tableSlug, viewId] as const,
  },
  /**
   * Раскладка карточки записи — порядок полей в drawer. Своя у каждого
   * пункта меню и не имеет отношения к колонкам view: таблица и drawer
   * показывают одну строку по-разному, и порядок у них разный.
   */
  layouts: {
    all: ["layouts"] as const,
    byMenu: (envId: string, tableSlug: string, menuId: string) =>
      [...keys.layouts.all, envId, tableSlug, menuId] as const,
  },
  items: {
    all: ["items"] as const,
    /** Все страницы и отборы одной таблицы: после импорта устаревают все. */
    table: (tableSlug: string) => [...keys.items.all, tableSlug] as const,
    list: (tableSlug: string, params: Record<string, unknown>) =>
      [...keys.items.all, tableSlug, params] as const,
    detail: (tableSlug: string, id: string) =>
      [...keys.items.all, tableSlug, "detail", id] as const,
    /**
     * Дети одного узла TREE view. `parent` пустой — корни. Набор полей
     * в ключе: ответ содержит ровно запрошенные колонки, и view с другим
     * набором не должен читать чужой кэш.
     */
    tree: (tableSlug: string, parent: string, fields: string) =>
      [...keys.items.all, tableSlug, "tree", parent, fields] as const,
  },
} as const;
