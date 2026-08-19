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
    relationRows: (tableSlug: string, search: string) =>
      [...keys.tables.all, "relation-rows", tableSlug, search] as const,
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
     * Права роли на пункты меню — по уровню дерева: бэкенд отдаёт их
     * по одному родителю, как и само меню.
     */
    menuPermissionsAll: () => [...keys.settings.all, "menu-permissions"] as const,
    menuPermissions: (projectId: string, roleId: string, parentId: string) =>
      [...keys.settings.menuPermissionsAll(), projectId, roleId, parentId] as const,
    /** Наборы значков iconify. Общие на всё приложение, а не на проект. */
    iconCollections: () => [...keys.settings.all, "icon-collections"] as const,
    /** Справочник: LANGUAGE, TIMEZONE, CURRENCY. Общий на проект. */
    options: (projectId: string, type: string) =>
      [...keys.settings.all, "options", projectId, type] as const,
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
  /** Функции проекта: их зовут поля-кнопки. Список один на окружение. */
  functions: {
    all: ["functions"] as const,
    list: (envId: string) => [...keys.functions.all, envId] as const,
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
  },
} as const;
