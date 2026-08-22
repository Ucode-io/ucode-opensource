import type { Field } from "@/features/table";

/**
 * Раскладка карточки записи (layout) — порядок полей в drawer.
 *
 * Это НЕ колонки view. Таблица показывает строку поперёк и хранит свой
 * порядок в `view.columns`; drawer показывает её вдоль и берёт порядок из
 * layout пункта меню. В старой админке так же (layoutService), и порядок
 * полей в карточке никогда не двигал колонки таблицы.
 *
 * Внутри layout поля разложены по секциям: `tabs[type=section]` →
 * `sections[]` → `fields[]`. Секции в drawer не рисуются — нужен только
 * порядок, — но при записи сохраняются как есть: PUT перезаписывает layout
 * целиком, и секции, которых нет в теле, бэкенд удаляет (layout.go, Update).
 *
 * Порядок — это позиция в массиве, а не поле `order`: бэкенд пишет `order`
 * из индекса при записи и читает секции `ORDER BY "order"`. Сортировать
 * ещё раз на клиенте нечего.
 */

/**
 * Поле в секции.
 *
 * `attributes.field_hide_layout` — «не показывать в карточке». Это
 * настройка РАСКЛАДКИ, а не поля: то же поле остаётся колонкой таблицы.
 * Ключ бывает не задан вовсе — тогда поле показывается.
 */
type LayoutField = {
  slug?: string;
  attributes?: {
    field_hide_layout?: boolean;
    /**
     * Права РОЛИ на это поле. Приходят только здесь: GET /v2/fields
     * их не отдаёт вовсе (в SELECT их нет), а раскладка подставляет
     * их по роли из токена — layout.go, `getFieldsWithPermissions`.
     */
    field_permission?: { view_permission?: boolean; edit_permission?: boolean };
  };
};
/**
 * Секция карточки: имя и поля.
 *
 * Имя приходится держать в ДВУХ местах. Колонку `section.label` PUT пишет
 * (layout.go, Update — она есть в списке колонок), но ни один GET её
 * не возвращает: запрос секций выбирает `id`, `"order"`, `fields`
 * и `attributes`, и всё (layout.go:1367, GetSections — тот же запрос
 * обслуживает и GET по пункту меню, и список раскладок). Секция,
 * переименованная в карточке, после перезапроса снова оказывалась
 * безымянной.
 *
 * Поэтому имя пишется и в колонку — ради старой админки и ради дня,
 * когда запрос починят, — и в `attributes.label`, который ездит туда
 * и обратно. Читается сначала из attributes.
 */
type SectionAttributes = { label?: string };
type LayoutSection = {
  label?: string;
  attributes?: SectionAttributes;
  fields?: LayoutField[];
};
/**
 * Связь вкладки, как её отдаёт ручка раскладки.
 *
 * Здесь она урезана до неузнаваемости: `GetRelation` (storage/postgres/
 * layout.go:1628) выбирает только `id`, `type`, `view_fields` и стороны,
 * а `relation_field_slug`, права и колонки не приходят ВООБЩЕ. Поэтому
 * настоящая связь берётся не отсюда, а из ручки связей таблицы
 * (GET /v2/relations/{slug}) — она уже загружена ради колонок-ссылок,
 * и в ней есть и `field_from`, и `relation_field_slug`.
 *
 * Остаются два применения: `permission` — на случай, если бэкенд
 * когда-нибудь начнёт его отдавать, и `columns` — прочитать колонки
 * у вкладок, заведённых до переезда настроек в `attributes`.
 */
type LayoutRelation = {
  id?: string;
  columns?: string[];
  title?: string;
  permission?: { view_permission?: boolean; create_permission?: boolean };
};

/** Настройки вкладки-секции. Вкладки связей живут не здесь — см. features/view. */
type TabAttributes = {
  /**
   * `layout_heading` — слаг поля, которое служит заголовком карточки.
   * У мультиязычной таблицы это не слаг, а карта «код языка → слаг»:
   * заголовок у каждого языка свой.
   */
  layout_heading?: string | Record<string, string>;
};

type LayoutTab = {
  id?: string;
  label?: string;
  type?: string;
  /** Колонка `tab.relation_id`: по ней бэкенд и связывает вкладку. */
  relation_id?: string;
  relation?: LayoutRelation;
  sections?: LayoutSection[];
  attributes?: TabAttributes;
};

/** Остальное тело layout не разбирается: оно уходит обратно в PUT как есть. */
export type Layout = Record<string, unknown> & { tabs?: LayoutTab[] };

/** Слаги полей в порядке карточки. Пусто — раскладки нет, порядок обычный. */
export function fieldOrder(layout: Layout | undefined): string[] {
  return flatFields(layout).map((field) => field.slug ?? "");
}

/**
 * Слаги полей, спрятанных из карточки (`field_hide_layout`).
 *
 * Отдаются отдельным списком, а не вычитаются из порядка: поле, которого
 * в раскладке нет вовсе, — это новое поле, и его надо ПОКАЗАТЬ (бэкенд
 * дописывает такие в последнюю секцию сам, но не мгновенно). Спрятано
 * только то, что спрятали явно.
 */
export function hiddenFields(layout: Layout | undefined): string[] {
  return flatFields(layout)
    .filter((field) => field.attributes?.field_hide_layout === true)
    .map((field) => field.slug ?? "")
    .filter(Boolean);
}

/**
 * Права роли на поля: что не показывать и что не давать править.
 *
 * Читаются из раскладки, потому что больше их взять негде. Схема полей
 * (GET /v2/fields) приходит одинаковой для всех ролей — колонок
 * `view_permission` и `edit_permission` в её запросе нет, — а раскладка
 * идёт за ними в `field_permission` по роли из токена.
 *
 * Запрет строгий, разрешение — по умолчанию: прячется только то, что
 * запрещено ЯВНО. Поля, которого в раскладке нет, это не касается,
 * и это не мелочь: запрос раскладки соединяется с правами так, что
 * у роли без единой записи в `field_permission` полей в ответе нет
 * ВООБЩЕ (layout.go:995 — условие на `fp.role_id` в WHERE превращает
 * LEFT JOIN во внутренний). Прячь мы «всё, чего нет в раскладке» —
 * такая роль видела бы пустую таблицу.
 */
export function fieldRights(layout: Layout | undefined): {
  hidden: Set<string>;
  readonly: Set<string>;
} {
  const hidden = new Set<string>();
  const readonly = new Set<string>();

  for (const field of flatFields(layout)) {
    const permission = field.attributes?.field_permission;
    if (!field.slug || !permission) continue;

    if (permission.view_permission === false) hidden.add(field.slug);
    if (permission.edit_permission === false) readonly.add(field.slug);
  }

  return { hidden, readonly };
}

/**
 * Колонки, приведённые к правам роли: запрещённых к показу нет вовсе,
 * запрещённые к правке — только для чтения.
 *
 * Одно место на всех: и колонки таблицы, и поля карточки растут из
 * одного списка, и прятать поле в одном из них значит показать его
 * в другом.
 *
 * Настоящую проверку делает сервер: правку скрытого поля он отклонит
 * и без нас. Здесь — чтобы человек не смотрел на колонку, которой
 * ему видеть не положено.
 */
export function applyRights(
  columns: Field[],
  rights: { hidden: Set<string>; readonly: Set<string> },
): Field[] {
  if (!rights.hidden.size && !rights.readonly.size) return columns;

  return columns
    .filter((field) => !rights.hidden.has(field.slug))
    .map((field) =>
      field.editable && rights.readonly.has(field.slug) ? { ...field, editable: false } : field,
    );
}

/**
 * Секции карточки: имя и слаги полей. Пусто — раскладки нет.
 *
 * Секция без имени — не ошибка, а норма: у карточки почти всегда одна
 * безымянная секция, и рисовать над ней пустой заголовок незачем.
 */
export function sections(layout: Layout | undefined): { label: string; slugs: string[] }[] {
  return (sectionTab(layout)?.sections ?? []).map((section) => ({
    label: sectionLabel(section),
    slugs: (section.fields ?? []).map((field) => field.slug ?? "").filter(Boolean),
  }));
}

/** Имя секции: сначала из attributes, потом из колонки — см. LayoutSection. */
function sectionLabel(section: LayoutSection): string {
  return (section.attributes?.label ?? section.label ?? "").trim();
}

/** Секция с новым именем — в обоих местах сразу. */
function named(section: LayoutSection, label: string): LayoutSection {
  return { ...section, label, attributes: { ...section.attributes, label } };
}

/**
 * Слаг поля-заголовка карточки.
 *
 * Задаётся админом и хранится в раскладке, а не берётся первой колонкой
 * view: колонки таблицы переставляют часто, и заголовок карточки уезжал
 * бы вместе с ними. Не задан — заголовка нет, и все поля идут списком.
 *
 * У мультиязычной таблицы значение — карта «язык → слаг»: заголовок
 * читается на активном языке, а не на каком придётся.
 */
export function headingSlug(layout: Layout | undefined, language: string): string {
  const heading = sectionTab(layout)?.attributes?.layout_heading;

  if (typeof heading === "string") return heading;
  if (!heading || typeof heading !== "object") return "";

  return heading[language] ?? Object.values(heading).find(Boolean) ?? "";
}

/**
 * Заголовок записи одной строкой — им подписана карточка в крошках.
 *
 * Заголовком карточки бывает только текст, но `layout_heading` хранится
 * слагом, и после смены типа поля там оказывается что угодно. Объект
 * и массив тогда встали бы в шапку как `[object Object]`; пусто честнее —
 * вместо него подставится запасная подпись.
 */
export function itemTitle(row: Record<string, unknown> | undefined, heading: string): string {
  const value = heading ? row?.[heading] : undefined;

  if (value === null || value === undefined || typeof value === "object") return "";

  return String(value);
}

/**
 * Раскладка с новым полем-заголовком.
 *
 * У мультиязычной таблицы пишется карта по всем языкам сразу: человек
 * выбирает поле один раз, а заголовок нужен на каждом языке — иначе
 * переключение языка обнуляло бы заголовок карточки.
 */
export function setHeading(
  layout: Layout,
  slug: string,
  variants: Record<string, string> | null,
): Layout {
  const tab = sectionTab(layout);
  if (!tab) return layout;

  const next = {
    ...tab,
    attributes: { ...tab.attributes, layout_heading: variants ?? slug },
  };

  return { ...layout, tabs: (layout.tabs ?? []).map((item) => (item === tab ? next : item)) };
}

/**
 * Колонки в порядке карточки. Поля, которых в layout нет, встают в конец
 * в своём порядке — сортировка устойчивая, и ранг у всех неизвестных один.
 */
export function orderColumns(columns: Field[], order: string[]): Field[] {
  if (!order.length) return columns;

  const at = new Map(order.map((slug, index) => [slug, index]));
  const rank = (field: Field) => at.get(field.slug) ?? Number.MAX_SAFE_INTEGER;

  return [...columns].sort((a, b) => rank(a) - rank(b));
}

/**
 * Layout с полем `moved`, переставленным к `target` — до или после.
 *
 * Чистая функция, потому что ошибка здесь тихая: тело уедет в PUT,
 * порядок в базе поменяется не так, как показали, и заметит это следующий,
 * кто откроет карточку.
 */
export function moveField(
  layout: Layout,
  moved: string,
  target: string,
  after: boolean,
): Layout {
  const tab = sectionTab(layout);
  if (!tab || moved === target) return layout;

  const flat = flatFields(layout);
  const from = flat.find((field) => field.slug === moved);
  const rest = flat.filter((field) => field !== from);
  const at = rest.findIndex((field) => field.slug === target);

  /*
   * Поля нет в раскладке — оставляем как есть. Новое поле бэкенд сам
   * дописывает в последнюю секцию (field.go, Create), так что это либо
   * гонка с соседней вкладкой, либо поле, удалённое из layout руками.
   */
  if (!from || at === -1) return layout;

  /*
   * Поле переезжает В СЕКЦИЮ цели, а не просто на её место в общем
   * списке. Раньше размеры секций сохранялись, и поле, бывшее последним
   * в первой секции, при переносе во вторую выталкивало оттуда соседа
   * назад — секции менялись местами по одному полю за раз.
   */
  const target_section = (tab.sections ?? []).findIndex((section) =>
    (section.fields ?? []).some((field) => field.slug === target),
  );

  const sections = (tab.sections ?? []).map((section, index) => {
    const fields = (section.fields ?? []).filter((field) => field.slug !== moved);
    if (index !== target_section) return { ...section, fields };

    const at_in = fields.findIndex((field) => field.slug === target);
    const cut = after ? at_in + 1 : at_in;
    return { ...section, fields: [...fields.slice(0, cut), from, ...fields.slice(cut)] };
  });

  return withSections(layout, tab, sections);
}

/** Новая секция в конце карточки. Пустая: поля в неё переносят мышью. */
export function addSection(layout: Layout, label: string): Layout {
  const tab = sectionTab(layout);
  if (!tab) return layout;

  return withSections(layout, tab, [...(tab.sections ?? []), named({ fields: [] }, label)]);
}

/** Переименование секции. Пустое имя — секция без заголовка, это законно. */
export function renameSection(layout: Layout, index: number, label: string): Layout {
  const tab = sectionTab(layout);
  if (!tab || !tab.sections?.[index]) return layout;

  return withSections(
    layout,
    tab,
    tab.sections.map((section, at) => (at === index ? named(section, label) : section)),
  );
}

/**
 * Удаление секции. Поля уходят в соседнюю — ту, что выше, а у первой
 * секции в ту, что ниже.
 *
 * Терять поля нельзя: они исчезли бы из карточки целиком, и вернуть их
 * можно было бы только правкой раскладки руками. Последнюю секцию
 * не удаляем по той же причине — полям некуда деться.
 */
export function removeSection(layout: Layout, index: number): Layout {
  const tab = sectionTab(layout);
  const sections = tab?.sections ?? [];
  if (!tab || sections.length < 2 || !sections[index]) return layout;

  const orphans = sections[index]?.fields ?? [];
  const into = index === 0 ? 1 : index - 1;

  const next = sections
    .map((section, at) =>
      at === into ? { ...section, fields: [...(section.fields ?? []), ...orphans] } : section,
    )
    .filter((_, at) => at !== index);

  return withSections(layout, tab, next);
}

/** Тот же layout с новым набором секций. Остальное тело не трогаем. */
function withSections(layout: Layout, tab: LayoutTab, sections: LayoutSection[]): Layout {
  return {
    ...layout,
    tabs: (layout.tabs ?? []).map((item) => (item === tab ? { ...tab, sections } : item)),
  };
}

const SECTION = "section";

/** Вкладка с полями записи. Остальные вкладки layout — это связи. */
function sectionTab(layout: Layout | undefined): LayoutTab | undefined {
  return layout?.tabs?.find((tab) => tab.type === SECTION);
}

function flatFields(layout: Layout | undefined): LayoutField[] {
  return (sectionTab(layout)?.sections ?? []).flatMap((section) => section.fields ?? []);
}
