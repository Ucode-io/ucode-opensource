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
type LayoutField = { slug?: string; attributes?: { field_hide_layout?: boolean } };
/** У секции есть имя: в старой админке она рисуется заголовком над группой полей. */
type LayoutSection = { label?: string; fields?: LayoutField[] };
/**
 * Связь, которую показывает вкладка: чужая таблица, колонка-ссылка в ней,
 * колонки для показа и права роли.
 *
 * Колонку-ссылку приходится выводить. `relation_field_slug` — настоящее
 * её имя, но в ответе layout его чаще всего нет вовсе: приезжают только
 * стороны связи (`table_from`, `table_to`) и `relation_table_slug`.
 * Тогда имя собирается по правилу, по которому его завёл бэкенд:
 * колонка в table_from называется `<слаг table_to>_id`
 * (pkg/helper/relation.go — `fieldFrom = data.TableTo + "_id"`).
 *
 * Это не угадывание: другого имени у колонки быть не может — вторую
 * связь на ту же таблицу бэкенд создать не даст, колонка уже занята.
 */
type LayoutRelation = {
  id?: string;
  relation_table_slug?: string;
  relation_field_slug?: string;
  /** Стороны связи. Приезжают развёрнутыми, а не слагами. */
  table_from?: { slug?: string };
  table_to?: { slug?: string };
  columns?: string[];
  title?: string;
  permission?: { view_permission?: boolean; create_permission?: boolean };
};

type LayoutTab = {
  id?: string;
  label?: string;
  type?: string;
  /** Колонка `tab.relation_id`: по ней бэкенд и связывает вкладку. */
  relation_id?: string;
  relation?: LayoutRelation;
  sections?: LayoutSection[];
  /**
   * `layout_heading` — слаг поля, которое служит заголовком карточки.
   * У мультиязычной таблицы это не слаг, а карта «код языка → слаг»:
   * заголовок у каждого языка свой.
   */
  attributes?: { layout_heading?: string | Record<string, string> };
};

/** Вкладка связи в карточке записи — то, что от неё нужно наружу. */
export type RelationTab = {
  id: string;
  /** Связь, которую показывает вкладка. По ней её и заводят. */
  relationId: string;
  label: string;
  /** Таблица, строки которой показывает вкладка. */
  tableSlug: string;
  /** Колонка-ссылка на открытую запись В ЭТОЙ таблице. */
  fieldSlug: string;
  /** Колонки вкладки: id полей, как в `view.columns`. */
  columnIds: string[];
  /** Можно ли создавать связанные строки. */
  canCreate: boolean;
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
 * Секции карточки: имя и слаги полей. Пусто — раскладки нет.
 *
 * Секция без имени — не ошибка, а норма: у карточки почти всегда одна
 * безымянная секция, и рисовать над ней пустой заголовок незачем.
 */
export function sections(layout: Layout | undefined): { label: string; slugs: string[] }[] {
  return (sectionTab(layout)?.sections ?? []).map((section) => ({
    label: section.label?.trim() ?? "",
    slugs: (section.fields ?? []).map((field) => field.slug ?? "").filter(Boolean),
  }));
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
 * Layout с новой вкладкой связи.
 *
 * Вкладки бэкенд перезаписывает списком целиком (storage/postgres/
 * layout.go:194 — bulk insert по всем `tabs`), поэтому добавить —
 * это дописать элемент. Связь указывается идентификатором: остальное
 * (таблицу, права, колонку-ссылку) он подставит сам, отдавая раскладку.
 *
 * id вкладки создаём мы: колонка `tab.id` приходит в теле и берётся
 * как есть, своего бэкенд не выдаёт.
 */
export function addRelationTab(
  layout: Layout,
  tab: { id: string; label: string; relationId: string },
): Layout {
  return {
    ...layout,
    tabs: [
      ...(layout.tabs ?? []),
      { id: tab.id, label: tab.label, type: "relation", relation_id: tab.relationId },
    ],
  };
}

/** Layout без вкладки. Строки чужой таблицы при этом никуда не деваются. */
export function removeTab(layout: Layout, tabId: string): Layout {
  return { ...layout, tabs: (layout.tabs ?? []).filter((tab) => tab.id !== tabId) };
}

/**
 * Layout с новым набором колонок у вкладки связи.
 *
 * Колонки вкладки живут в самой раскладке (`tabs[].relation.columns`),
 * а не в отдельном view: у вкладки нет ни своего адреса, ни своих
 * фильтров, и заводить ради списка колонок вторую сущность незачем.
 *
 * Вкладка, которой в раскладке нет, возвращается как есть: значит
 * раскладку успели перезапросить, и правка относилась к прежней.
 */
export function setTabColumns(layout: Layout, tabId: string, columnIds: string[]): Layout {
  return {
    ...layout,
    tabs: (layout.tabs ?? []).map((tab) =>
      tab.id === tabId && tab.relation
        ? { ...tab, relation: { ...tab.relation, columns: columnIds } }
        : tab,
    ),
  };
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

  const cut = after ? at + 1 : at;
  const next = [...rest.slice(0, cut), from, ...rest.slice(cut)];

  // Размеры секций сохраняются: в drawer их не видно, но в старой админке
  // они рисуют карточку, и схлопывать всё в одну секцию нельзя.
  let taken = 0;
  const sections = (tab.sections ?? []).map((section) => {
    const size = section.fields?.length ?? 0;
    const fields = next.slice(taken, taken + size);
    taken += size;
    return { ...section, fields };
  });

  return {
    ...layout,
    tabs: (layout.tabs ?? []).map((item) => (item === tab ? { ...tab, sections } : item)),
  };
}

/**
 * Вкладки связей карточки: связанные строки рядом с полями записи.
 *
 * Источник — раскладка, а не список view пункта меню, хотя relation view
 * там тоже лежат. В раскладке у вкладки есть всё нужное разом: имя
 * колонки-ссылки, права роли на просмотр и создание, колонки для показа.
 * В списке view нет ни колонки-ссылки, ни прав.
 *
 * Права читаются строго: право есть, пока его явно не отняли. Блок
 * `permission` приходит не отовсюду, и его отсутствие означает «прав
 * не настраивали», а не «нельзя» — иначе вкладки исчезли бы у всех
 * проектов, где матрицу прав не трогали.
 *
 * Вкладка без чужой таблицы или без колонки-ссылки пропускается:
 * отбирать строки нечем, и она показала бы всю таблицу целиком.
 */
export function relationTabs(layout: Layout | undefined): RelationTab[] {
  return (layout?.tabs ?? [])
    .filter((tab) => tab.type !== SECTION && tab.relation?.permission?.view_permission !== false)
    .map((tab) => ({
      id: tab.id ?? "",
      relationId: tab.relation_id ?? tab.relation?.id ?? "",
      label: tab.label?.trim() || tab.relation?.title?.trim() || tab.relation?.relation_table_slug || "—",
      tableSlug: tab.relation?.relation_table_slug ?? "",
      fieldSlug: linkField(tab.relation),
      columnIds: tab.relation?.columns ?? [],
      canCreate: tab.relation?.permission?.create_permission !== false,
    }))
    .filter((tab) => tab.id && tab.tableSlug && tab.fieldSlug);
}

/**
 * Колонка-ссылка на НАШУ запись в чужой таблице.
 *
 * Готовое имя, если бэкенд его прислал; иначе — по правилу создания
 * связи: колонка в table_from называется `<слаг table_to>_id`. Наша
 * сторона — та, которая не `relation_table_slug`.
 */
function linkField(relation: LayoutRelation | undefined): string {
  if (!relation) return "";
  if (relation.relation_field_slug) return relation.relation_field_slug;

  const from = relation.table_from?.slug ?? "";
  const to = relation.table_to?.slug ?? "";
  const own = relation.relation_table_slug === from ? to : from;

  return own ? `${own}_id` : "";
}

const SECTION = "section";

/** Вкладка с полями записи. Остальные вкладки layout — это связи. */
function sectionTab(layout: Layout | undefined): LayoutTab | undefined {
  return layout?.tabs?.find((tab) => tab.type === SECTION);
}

function flatFields(layout: Layout | undefined): LayoutField[] {
  return (sectionTab(layout)?.sections ?? []).flatMap((section) => section.fields ?? []);
}
