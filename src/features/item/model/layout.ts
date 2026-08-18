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
/** У секции есть имя: в старой админке она рисуется заголовком над группой полей. */
type LayoutSection = { label?: string; fields?: LayoutField[] };
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

const SECTION = "section";

/** Вкладка с полями записи. Остальные вкладки layout — это связи. */
function sectionTab(layout: Layout | undefined): LayoutTab | undefined {
  return layout?.tabs?.find((tab) => tab.type === SECTION);
}

function flatFields(layout: Layout | undefined): LayoutField[] {
  return (sectionTab(layout)?.sections ?? []).flatMap((section) => section.fields ?? []);
}
