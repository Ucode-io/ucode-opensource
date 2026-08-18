import { hexToChipColor, type ChipColor } from "@/shared/ui/chip";
import { slugify } from "@/shared/lib/slug";
import { STATUS_GROUPS, localized, type Field, type FieldOption, type StatusGroup } from "./types";

/**
 * Новое поле, как его заполняет человек. Форма ввода, а не тело запроса:
 * в тело это превращается один раз, в api/fields.
 */
export type DraftOption = {
  label: string;
  color: ChipColor;
  /**
   * То, что уже лежит в строках под этим вариантом.
   *
   * У нового варианта его нет — значение придумает api/fields. У
   * существующего оно ОБЯЗАНО пережить переименование: значение варианта
   * и его подпись — разные вещи, и вычислять первое из второй значит
   * осиротить все строки, где вариант уже проставлен. Ровно это и
   * происходило: переименование «Progress» рвало связь со значением
   * «Progress_slug», и ячейка теряла и цвет, и подпись.
   */
  value?: string;
};

/**
 * Агрегат FORMULA: «сумма поля X по связанным строкам таблицы Y».
 *
 * Считает бэкенд, и имена ключей в attributes историчны — здесь они
 * разобраны один раз (см. toAggregate и toSettingsAttributes):
 *
 *   attributes.type              SUMM | MAX | AVG
 *   attributes.table_from        «слаг#id связи» одной строкой
 *   attributes.sum_field         слаг поля, по которому считают
 *   attributes.number_of_rounds  знаков после запятой
 *   attributes.formula_filters   какие строки брать
 */
export const AGGREGATES = ["SUMM", "MAX", "AVG"] as const;
export type AggregateType = (typeof AGGREGATES)[number];

/**
 * Условие отбора строк. `key` — составной, «слаг#ТИП#таблица»: так его
 * пишет старая админка и так его читает бэкенд, поэтому наружу отдаётся
 * как есть, а разбирается в filterParts.
 */
export type AggregateFilter = { key: string; value: unknown };

export type Aggregate = {
  type: AggregateType | "";
  /** «слаг#id связи». Пусто — таблица не выбрана. */
  tableFrom: string;
  field: string;
  /** Знаков после запятой. Строка, потому что это поле ввода. */
  rounds: string;
  filters: AggregateFilter[];
};

export const EMPTY_AGGREGATE: Aggregate = {
  type: "",
  tableFrom: "",
  field: "",
  rounds: "",
  filters: [],
};

/** «слаг#ТИП#таблица» → части. Чужого формата больше никто не знает. */
export function filterParts(key: string): { slug: string; type: string; tableSlug: string } {
  const [slug = "", type = "", tableSlug = ""] = key.split("#");
  return { slug, type, tableSlug };
}

/**
 * Слаг таблицы из составного значения «слаг#…». Второй частью бэкенд
 * кладёт разное — id связи у агрегата, колонку-связь у автозаполнения, —
 * а первой всегда слаг чужой таблицы.
 */
export function tableFromSlug(tableFrom: string): string {
  return tableFrom.split("#")[0] ?? "";
}

export type FieldDraft = {
  label: string;
  /** Имя колонки в базе. Подставляется из названия, но правится руками. */
  slug: string;
  type: string;
  /** Варианты MULTISELECT. */
  options: DraftOption[];
  /** Варианты STATUS: у него три стадии, и это не оформление, а схема. */
  groups: Record<StatusGroup, DraftOption[]>;
  /**
   * Заполнение обязательно. Ставится только правкой: при создании
   * бэкенд пишет в колонку жёсткий false (field.go, третий аргумент
   * INSERT'а), и переключатель в форме нового поля ничего бы не значил.
   */
  required: boolean;
  /** Уникальное. При создании превращается в UNIQUE-ограничение в базе. */
  unique: boolean;
  /** Только чтение: тот самый attributes.disabled, который читает схема. */
  readonly: boolean;
  /**
   * Регулярное выражение, которому обязано соответствовать значение,
   * и текст отказа. Проверяет только фронт — бэкенд эти ключи не читает
   * (attributes для него свободный JSONB).
   */
  validation: string;
  validationMessage: string;
  /**
   * FORMULA_FRONTEND: выражение по слагам полей ТОЙ ЖЕ строки, которое
   * считает браузер. Excel-подобный синтаксис — так его понимает
   * hot-formula-parser, которым старая админка считала эти поля,
   * и написанные формулы должны продолжать работать.
   */
  formula: string;
  /** FORMULA: агрегат по связанной таблице, его считает бэкенд. */
  aggregate: Aggregate;
  /**
   * Сколько цифр в номере у INCREMENT_ID. Пусто — девять, как у бэкенда
   * по умолчанию. Число задаёт верхнюю границу последовательности
   * (10^n − 1) и учитывается ровно в момент, когда поле заводится.
   */
  digits: string;
  /**
   * Автозаполнение: значение берётся не у человека, а из строки,
   * на которую указывает связь.
   *
   *   autofillTable  «чужая таблица#колонка-связь В ЭТОЙ таблице»,
   *                  например «clients#client_id». Ровно в таком виде
   *                  это лежит в колонке и так его разбирает бэкенд
   *                  (object_builder.go: `strings.Split(..., "#")[1]`)
   *   autofillField  слаг поля ЧУЖОЙ таблицы, откуда берётся значение
   *   automatic      подставлять сразу, а не только в пустое поле
   *
   * Это три КОЛОНКИ таблицы field, а не ключи attributes. В attributes
   * они тоже приходят — бэкенд их туда копирует, отдавая layout
   * (layout.go: `attributes["autofill_table"] = ...`), — но это эхо:
   * PUT читает и пишет только колонки.
   */
  autofillTable: string;
  autofillField: string;
  automatic: boolean;
  /**
   * Значение хранится по языкам проекта (см. CONTEXT, Multilanguage
   * Field). Признак — КОЛОНКА `enable_multilanguage`, и только она:
   * `attributes.enable_multilanguage` бэкенд дописывает сам при отдаче
   * (object_builder.go), а `enable_multi_language` не существует вовсе.
   *
   * При создании флаг делает не то же, что при правке: шлюз заводит
   * по отдельному полю на каждый язык проекта — `title_en`, `title_cyr`
   * (field.go: SetTitlePrefix), — и только для SINGLE_LINE и MULTI_LINE.
   * Сама колонка при этом остаётся false: INSERT в object_builder её
   * не перечисляет. Правка пишет её как есть.
   */
  multilanguage: boolean;
  /**
   * Подписи по языкам ДАННЫХ проекта: `attributes.label_<код>`.
   *
   * Это НЕ мультиязычное значение поля (то — отдельные колонки, см.
   * multilanguage): подпись у колонки одна, просто написана на разных
   * языках. `label` — та же подпись на активном языке, и одна из этих
   * двух вещей всегда дублирует другую: колонку `label` бэкенд хранит
   * отдельно от attributes.
   */
  labels: Record<string, string>;
  /**
   * BUTTON: колонки со значением у него нет вовсе — есть иконка и
   * функция, которую зовёт клик.
   *
   *   icon        attributes.icon, тот же формат, что у пунктов меню:
   *               «tabler:bolt», ссылка или файл в нашем CDN
   *   functionId  attributes.function — id функции, а не путь
   *
   * Оба ключа в attributes, а не колонками: бэкенд про них не знает
   * ничего, кроме того, что отдаёт их обратно как есть. Зовёт функцию
   * фронт — POST /v1/invoke_function (см. features/item/api/functions).
   */
  icon: string;
  functionId: string;
};

/**
 * Типы, у которых значение бывает на нескольких языках. Список не наш:
 * ровно по нему шлюз решает, разводить ли поле по колонке на язык
 * (field.go: CreateField). У остальных флаг завёл бы одну колонку
 * с обещанием, которого никто не выполняет.
 */
export const MULTILANGUAGE_TYPES = new Set(["SINGLE_LINE", "MULTI_LINE"]);

export const EMPTY_DRAFT: FieldDraft = {
  label: "",
  slug: "",
  type: "SINGLE_LINE",
  options: [],
  groups: { todo: [], progress: [], complete: [] },
  required: false,
  unique: false,
  readonly: false,
  validation: "",
  validationMessage: "",
  formula: "",
  aggregate: EMPTY_AGGREGATE,
  digits: "",
  autofillTable: "",
  autofillField: "",
  automatic: false,
  multilanguage: false,
  labels: {},
  icon: "",
  functionId: "",
};

/**
 * Существующее поле → черновик формы.
 *
 * Подписи вариантов берутся на языке ДАННЫХ, а цвет — обратным разбором
 * HEX в оттенок палитры: в форме выбирают оттенок, в базе лежит HEX
 * (см. hexToChipColor). Круг замыкается: открыл, ничего не тронул,
 * сохранил — данные те же.
 */
export function toDraft(field: Field, language: string): FieldDraft {
  const options = [...field.options.values()];
  const draft: FieldDraft = {
    // Ровно та подпись, что стоит в шапке колонки: у поля их две —
    // колонка `label` и перевод на язык данных, и правят видимую.
    label: localized(field.labels, language, field.label),
    // Подписи на всех языках проекта: их правит одно поле с переключателем.
    labels: { ...field.labels },
    slug: field.slug,
    type: field.type,
    options: [],
    groups: { todo: [], progress: [], complete: [] },
    // Из ответа сервера как есть: required и unique — колонки таблицы
    // field, «только чтение» — флаг в attributes.
    required: field.raw["required"] === true,
    unique: field.raw["unique"] === true,
    readonly: field.attributes["disabled"] === true,
    // Исходный текст выражения, а не собранный RegExp: править человек
    // будет ровно то, что написал, включая незакрытую скобку.
    validation: textOf(field.attributes["validation"]),
    validationMessage: textOf(field.attributes["validation_message"]),
    formula: textOf(field.attributes["formula"]),
    aggregate: toAggregate(field.attributes),
    digits: digitsOf(field.attributes),
    /*
     * Автозаполнение и мультиязычность читаются из КОЛОНОК ответа,
     * как required и unique. Одноимённые ключи в attributes — эхо,
     * которое бэкенд дописывает при отдаче layout: у поля, заведённого
     * не через layout, их там просто нет, а после правки они разъезжаются
     * с колонками. Истина одна, и она в колонке (см. CONTEXT).
     */
    autofillTable: textOf(field.raw["autofill_table"]),
    autofillField: textOf(field.raw["autofill_field"]),
    automatic: field.raw["automatic"] === true,
    multilanguage: field.raw["enable_multilanguage"] === true,
    icon: textOf(field.attributes["icon"]),
    functionId: textOf(field.attributes["function"]),
  };

  if (optionsShape(field.type) === "groups") {
    for (const option of options) {
      // Вариант без стадии в схеме STATUS невозможен, но данные
      // переживают смену типа поля: такой кладём в первую стадию,
      // а не теряем.
      draft.groups[option.group ?? "todo"].push(toDraftOption(option, language));
    }
    return draft;
  }

  draft.options = options.map((option) => toDraftOption(option, language));
  return draft;
}

function textOf(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/**
 * Настройки агрегата из attributes.
 *
 * Всё читается мягко: поле могло быть заведено старой админкой, и там
 * половина ключей проставляется только после того, как человек дошёл
 * до нужного шага. Незаполненное — пустая строка, а не undefined:
 * дальше это значения полей ввода.
 */
function toAggregate(attributes: Record<string, unknown>): Aggregate {
  const type = textOf(attributes["type"]);
  const rounds = attributes["number_of_rounds"];
  const filters = attributes["formula_filters"];

  return {
    type: (AGGREGATES as readonly string[]).includes(type) ? (type as AggregateType) : "",
    tableFrom: textOf(attributes["table_from"]),
    field: textOf(attributes["sum_field"]),
    rounds: typeof rounds === "number" || typeof rounds === "string" ? String(rounds) : "",
    filters: Array.isArray(filters)
      ? filters
          .filter((item): item is Record<string, unknown> => typeof item === "object" && item !== null)
          .map((item) => ({ key: textOf(item["key"]), value: item["value"] ?? null }))
      : [],
  };
}

/**
 * Компилируется ли выражение. Набранное с ошибкой сохранить можно —
 * тело запроса от этого не сломается, — но проверять им ничего не будут
 * (normalize такое выражение отбрасывает), и сказать об этом надо в поле
 * ввода, а не молчать.
 */
export function isValidPattern(source: string): boolean {
  try {
    new RegExp(source);
    return true;
  } catch {
    return false;
  }
}

/** Число цифр приходит то числом, то строкой — в форме оно строка. */
function digitsOf(attributes: Record<string, unknown>): string {
  const value = attributes["digit_number"];
  return typeof value === "number" || typeof value === "string" ? String(value) : "";
}

function toDraftOption(option: FieldOption, language: string): DraftOption {
  return {
    label: localized(option.labels, language, option.label || option.value),
    color: option.color ? hexToChipColor(option.color) : "gray",
    // Ключ поиска после нормализации один у обоих типов (см. CONTEXT):
    // у MULTISELECT это slug, у STATUS — value.
    value: option.value,
  };
}

/**
 * Варианты, с которыми поле заводится.
 *
 * У STATUS их три, и это не украшение: поле без вариантов не выбирается
 * вообще (editorKind закрывает пустой выбор), то есть свежесозданная
 * колонка была бы мёртвой до похода в настройки. Стадии заведены по
 * одному варианту на каждую — ровно то, ради чего у типа три списка.
 *
 * Подписи английские: это ЗНАЧЕНИЕ в данных, а не текст интерфейса.
 * Язык данных задаёт проект, и переводить их локалью админки нельзя.
 */
export function defaultGroups(type: string): Record<StatusGroup, DraftOption[]> {
  if (type !== "STATUS") return { todo: [], progress: [], complete: [] };

  return {
    todo: [{ label: "Not started", color: "gray" }],
    progress: [{ label: "In progress", color: "blue" }],
    complete: [{ label: "Done", color: "green" }],
  };
}

/**
 * Новое поле выбранного типа.
 *
 * Имя необязательно: тип выбирают кликом, и требовать перед этим ввести
 * название — лишний шаг. Без имени поле называется своим типом, как
 * в Notion; повтор разводится числом, потому что слаг станет именем
 * колонки в SQL, а второй такой же — это 500 от базы.
 */
export function newDraft(type: string, name: string, taken: Field[]): FieldDraft {
  const label = name.trim() || unique(fieldTypeLabel(type), taken.map((field) => field.label), " ");
  const slug = unique(slugify(label) || "field", taken.map((field) => field.slug), "_");

  return { ...EMPTY_DRAFT, label, slug, type, groups: defaultGroups(type) };
}

/**
 * Смена типа у черновика. Списки вариантов не стираются — у двух типов
 * они лежат в разных полях черновика, и вернувшийся обратно тип находит
 * свои варианты на месте. Пустые стадии STATUS заполняются умолчаниями
 * по той же причине, что и при создании.
 */
export function withType(draft: FieldDraft, type: string): FieldDraft {
  const empty = STATUS_GROUPS.every((group) => draft.groups[group].length === 0);

  return { ...draft, type, groups: empty ? defaultGroups(type) : draft.groups };
}

/** «Status», «Status 2», «Status 3» — первое свободное. */
export function unique(base: string, taken: string[], separator: string): string {
  if (!taken.includes(base)) return base;

  for (let n = 2; ; n++) {
    const candidate = `${base}${separator}${n}`;
    if (!taken.includes(candidate)) return candidate;
  }
}

/** Нужен ли типу список вариантов и какой. */
export function optionsShape(type: string): "flat" | "groups" | null {
  if (type === "MULTISELECT") return "flat";
  if (type === "STATUS") return "groups";
  return null;
}

/**
 * Типы, которые можно завести из таблицы.
 *
 * Список короче полного справочника ucode, и это выбор, а не недоделка.
 * Сюда попадает только то, что таблица умеет и показать, и объяснить:
 *
 *   не попали  DYNAMIC, LANGUAGE_TYPE, MONEY, ARRAY — нет ни рендера,
 *              ни договорённости о содержимом
 *
 * LOOKUP в списке есть, но обычным типом не является: выбор открывает
 * форму связи, а не создаёт колонку. Связь заводится своей ручкой
 * (POST /v2/relations), и колонку-ссылку бэкенд добавляет сам. Отсюда
 * и `RELATION_TYPE` — на него в форме одна явная проверка вместо
 * ветвления по всему списку.
 *
 * Порядок групп — от частого к редкому: поле заводят на бегу, и первое,
 * что видно, должно закрывать девять случаев из десяти.
 */
export const RELATION_TYPE = "LOOKUP";

/**
 * Группа типов.
 *
 * На экране групп не видно: список рисуется сплошным, в две колонки
 * (см. TypeList). Группы задают ПОРЯДОК — от частого к редкому — и
 * дают за что зацепиться, когда набор типов нужно поменять. `key`
 * при этом остаётся именем, по которому группу выбирают в коде:
 * «Связь» показывается не всем.
 */
export type FieldTypeGroup = {
  key: "text" | "choice" | "number" | "date" | "file" | "other" | "current" | "relation";
  types: { type: string; label: string }[];
};

export const FIELD_TYPE_GROUPS: FieldTypeGroup[] = [
  {
    key: "text",
    types: [
      { type: "SINGLE_LINE", label: "Single line" },
      { type: "MULTI_LINE", label: "Multi line" },
      { type: "EMAIL", label: "Email" },
      { type: "PHONE", label: "Phone" },
      { type: "INTERNATION_PHONE", label: "International phone" },
      { type: "LINK", label: "Link" },
      { type: "PASSWORD", label: "Password" },
    ],
  },
  {
    key: "relation",
    types: [{ type: RELATION_TYPE, label: "Relation" }],
  },
  {
    key: "choice",
    types: [
      { type: "STATUS", label: "Status" },
      { type: "MULTISELECT", label: "Multiselect" },
      { type: "CHECKBOX", label: "Checkbox" },
      { type: "SWITCH", label: "Switch" },
    ],
  },
  {
    key: "number",
    types: [
      { type: "NUMBER", label: "Number" },
      { type: "FLOAT", label: "Float" },
      { type: "INCREMENT_ID", label: "Increment id" },
    ],
  },
  {
    key: "date",
    types: [
      { type: "DATE", label: "Date" },
      { type: "DATE_TIME", label: "Date and time" },
      // «local», а не «no time zone»: в две колонки длинная подпись
      // обрезается ровно на той части, которая её и различает.
      { type: "DATE_TIME_WITHOUT_TIME_ZONE", label: "Date and time (local)" },
      { type: "TIME", label: "Time" },
    ],
  },
  {
    key: "file",
    types: [
      { type: "PHOTO", label: "Photo" },
      { type: "MULTI_IMAGE", label: "Photos" },
      { type: "FILE", label: "File" },
      { type: "MULTI_FILE", label: "Files" },
      { type: "VIDEO", label: "Video" },
    ],
  },
  {
    key: "other",
    types: [
      { type: "FORMULA_FRONTEND", label: "Formula" },
      { type: "FORMULA", label: "Aggregate" },
      { type: "BUTTON", label: "Button" },
      { type: "JSON", label: "JSON" },
      { type: "COLOR", label: "Color" },
      { type: "ICON", label: "Icon" },
      { type: "MAP", label: "Map" },
      { type: "POLYGON", label: "Polygon" },
    ],
  },
];

/** Подпись типа для списка. Незнакомый показываем как есть. */
export function fieldTypeLabel(type: string): string {
  for (const group of FIELD_TYPE_GROUPS) {
    const found = group.types.find((item) => item.type === type);
    if (found) return found.label;
  }
  return type;
}

/** Пустые стадии STATUS не отправляем, но перебирать их надо в одном порядке. */
export { STATUS_GROUPS };
export type { StatusGroup };

/**
 * Группы типов → блоки, разделённые чертой.
 *
 * Черта идёт во всю ширину и обязана начинать новую строку. Поэтому
 * блок закрывается ТОЛЬКО на чётном числе типов: у группы с нечётным
 * (текст — семь, числа — три) последняя строка занята наполовину,
 * и черта под ней оставила бы дыру в полстроки. Такая группа
 * склеивается со следующей, пока сумма не станет чётной.
 *
 * Отсюда и вид списка: разделены не все группы, а те границы, что
 * попали на границу строки. Считается, а не проставлено руками, —
 * иначе новый тип в списке молча возвращал бы дыру.
 *
 * Последний блок может остаться нечётным: полстроки в самом низу
 * списка — это не дыра, а его конец.
 */
export function toBlocks(groups: TypeItem[][]): TypeItem[][] {
  const blocks: TypeItem[][] = [];
  let block: TypeItem[] = [];

  for (const group of groups) {
    block = [...block, ...group];

    if (block.length % 2 === 0) {
      blocks.push(block);
      block = [];
    }
  }

  if (block.length) blocks.push(block);

  return blocks;
}

/** Один тип в списке выбора. */
export type TypeItem = { type: string; label: string };
