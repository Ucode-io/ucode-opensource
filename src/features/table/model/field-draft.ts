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
   * Приставка к автономеру: `INV-000123`. Только у типов, где номер
   * генерирует бэкенд.
   */
  prefix: string;
  /**
   * Значение по умолчанию у новой записи (`attributes.defaultValue`).
   *
   * Подставляет его ФРОНТ, а не база: колонки `default` у поля нет,
   * и бэкенд про эту настройку не знает вовсе — он просто хранит
   * её в свободном мешке attributes. Отсюда и camelCase посреди
   * змеиных имён: ключ придумала старая админка, и читает она
   * только его.
   */
  defaultValue: string;
  /**
   * Карта и область: точка, с которой открывается пустая ячейка.
   *
   * То же имя, что в старой админке: пара координат, а не адрес.
   * Ключ карт (`apiKey`) она спрашивает у каждого поля MAP отдельно —
   * мы не спрашиваем: карту рисуем сами и своим ключом
   * (`shared/lib/yandex-maps`), а чужой лежал бы в форме без читателя
   * (docs/FIELD-AUDIT.md, F25).
   */
  lat: string;
  long: string;
  /**
   * PHOTO: в каких пропорциях обрезать снимок при загрузке. Лежит
   * ЧИСЛОМ-строкой — «1.3» это 4:3: так её записывает старая админка,
   * деля одно на другое, и так её читает бэкенд (`file.go:66`).
   *
   * Соседний `format` (png/webp) не спрашиваем: его не читает ни ручка
   * загрузки, ни кто-либо ещё — снимок кодируется обратно в том
   * формате, в котором пришёл (F21).
   */
  ratio: string;
  /** VIDEO: перекодировать загруженное на сервере. */
  transcode: boolean;
  /**
   * Сканер штрихкодов (SCAN_BARCODE): отправлять по Enter, а не
   * по каждому символу, и сколько символов в коде.
   */
  pressEnter: boolean;
  length: string;
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
  /**
   * Условная видимость в карточке: показывать поле, только когда
   * значение СОСЕДНЕГО поля совпало с заданным. Применяет её
   * `features/item/model/visibility` — там же разобран формат и
   * названо то, чем он обманывает.
   *
   *   hideField    attributes.hide_path_field — слаг поля, за которым
   *                следим. Пусто — правила нет
   *   hideValues   attributes.hide_path — ожидаемое значение. Внутри
   *                всегда список: у MULTISELECT оно набором, у
   *                остальных из одного значения
   *   hideMulti    было ли оно набором. Хранится отдельно, потому что
   *                набор из одного варианта и одиночное значение
   *                в attributes выглядят по-разному, а различить их
   *                при отправке больше нечем
   *   hideCompare  attributes.type — «min» | «max» у числового поля.
   *                Пусто — сравнение на равенство
   */
  hideField: string;
  hideValues: string[];
  hideMulti: boolean;
  hideCompare: string;
};

/** Сравнения числового условия видимости. Границы ИСКЛЮЧАЮЩИЕ. */
export const HIDE_COMPARISONS = ["min", "max"] as const;

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
  prefix: "",
  defaultValue: "",
  lat: "",
  long: "",
  ratio: "",
  transcode: false,
  pressEnter: false,
  length: "",
  autofillTable: "",
  autofillField: "",
  automatic: false,
  multilanguage: false,
  labels: {},
  icon: "",
  functionId: "",
  hideField: "",
  hideValues: [],
  hideMulti: false,
  hideCompare: "",
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
    prefix: textOf(field.attributes["prefix"]),
    /*
     * `default_values` — прежнее имя того же ключа: старая админка
     * читает оба, а пишет `defaultValue`. Читаем так же, пишем только
     * новое — иначе значение осталось бы в двух местах и разъехалось.
     */
    defaultValue:
      textOf(field.attributes["defaultValue"]) || textOf(field.attributes["default_values"]),
    lat: numericOf(field.attributes["lat"]),
    long: numericOf(field.attributes["long"]),
    ratio: numericOf(field.attributes["ratio"]),
    transcode: field.attributes["transcode"] === true,
    pressEnter: field.attributes["pressEnter"] === true,
    length: numericOf(field.attributes["length"]),
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
    hideField: textOf(field.attributes["hide_path_field"]),
    hideValues: hideValuesOf(field.attributes["hide_path"]),
    hideMulti: Array.isArray(field.attributes["hide_path"]),
    /*
     * Числовое сравнение делит ключ `type` с видом агрегата FORMULA,
     * поэтому берутся ровно две известные строки: у поля-агрегата там
     * лежит SUMM, и принять его за границу нельзя.
     */
    hideCompare: HIDE_COMPARISONS.find((item) => item === field.attributes["type"]) ?? "",
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
 * Ожидаемое значение условия видимости — всегда списком. В attributes
 * оно бывает и строкой, и массивом; число сюда попадает от того, кто
 * записал его числом, — приводим, форма работает со строками.
 */
function hideValuesOf(value: unknown): string[] {
  const list = Array.isArray(value) ? value : [value];

  return list
    .filter((item) => item !== null && item !== undefined && item !== "")
    .map((item) => String(item));
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
/**
 * Число из attributes — строкой, потому что правится оно полем ввода.
 *
 * Числом его кладёт старая админка, строкой — её же старые версии:
 * координаты и пропорция встречаются в обоих видах, и разбирать
 * их по-разному значит потерять половину живых настроек.
 */
function numericOf(value: unknown): string {
  return typeof value === "number" || typeof value === "string" ? String(value) : "";
}

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
  if (type === "MULTISELECT" || type === "PICK_LIST") return "flat";
  if (type === "STATUS") return "groups";
  return null;
}

/**
 * Есть ли у типа приставка к автономеру.
 *
 * Список — по бэкенду, а не по старой админке: приставку подставляет
 * он сам, генерируя значение при вставке (helper/prepareFunctions.go —
 * INCREMENT_ID, RANDOM_NUMBERS, RANDOM_TEXT). Старая админка рисует
 * то же поле ещё у INCREMENT_NUMBER и CODABAR, где оно не делает
 * ничего: INCREMENT_NUMBER — обычный SERIAL, CODABAR бэкенд не
 * генерирует вовсе.
 */
export function hasPrefix(type: string): boolean {
  return type === "INCREMENT_ID" || type === "RANDOM_NUMBERS" || type === "RANDOM_TEXT";
}

/**
 * Типы, у которых спрашивается значение по умолчанию.
 *
 * Список разрешающий, а не запрещающий, и это осознанно. Значение
 * вводится одной строкой, а attributes перезаписываются целиком: у
 * MULTISELECT старая админка кладёт в тот же ключ СПИСОК, и покажи мы
 * ему текстовое поле — чужая настройка стёрлась бы при первом открытии
 * панели. Тип, которого тут нет, свой `defaultValue` сохраняет
 * нетронутым.
 */
const DEFAULT_VALUE_TYPES = new Set([
  "SINGLE_LINE",
  "MULTI_LINE",
  // TEXT здесь нет: у него показывается подпись поля, а не значение
  // записи (FIELD-AUDIT, F3), и подставлять в такую колонку нечего.
  "EMAIL",
  "PHONE",
  "INTERNATION_PHONE",
  "CODE",
  "COLOR",
  "ICON",
  "NUMBER",
  "FLOAT",
  "FLOAT_NOLIMIT",
  "CHECKBOX",
  "SWITCH",
  "DATE",
  "DATE_TIME",
  "DATE_TIME_WITHOUT_TIME_ZONE",
  "TIME",
  // Значение варианта, а не подпись: в строке лежит slug.
  "STATUS",
  "PICK_LIST",
]);

/**
 * Типы, у которых `digit_number` — это ДЛИНА генерируемого значения,
 * а не разрядность последовательности.
 *
 * Разница видна в том, когда настройка работает. У INCREMENT_ID число
 * задаёт верхнюю границу последовательности и учитывается ровно в момент
 * создания поля; здесь его читают на КАЖДОЙ вставке, поэтому правится
 * оно и у заведённого поля.
 */
export function hasLength(type: string): boolean {
  return type === "RANDOM_TEXT" || type === "RANDOM_NUMBERS";
}

/**
 * Длина генерируемого значения по умолчанию.
 *
 * Ноль здесь не «не задано», а поломка: `GenerateRandomString(prefix, 0)`
 * при пустой приставке возвращает пустую строку, а вызывающий цикл
 * крутится, пока она пуста (prepareFunctions.go:59) — вставка не падает,
 * а виснет. Поэтому длина у таких полей отправляется всегда.
 */
export const DEFAULT_LENGTH = 6;

/** Больше не влезает в int64, из которого бэкенд берёт диапазон чисел. */
export const MAX_LENGTH = 18;

export function hasDefaultValue(type: string): boolean {
  return DEFAULT_VALUE_TYPES.has(type);
}

/**
 * Настройки, которые есть только у одного-двух типов, — и у каких.
 *
 * Список тот же, что в старой админке (FieldSettings/Attributes),
 * потому что имена ключей придумала она, а читают их проекты:
 * загрузчик файлов, сканер на складе, карта в мобильном приложении.
 */
export function hasMapSettings(type: string): boolean {
  return type === "MAP" || type === "POLYGON";
}

export function hasPhotoSettings(type: string): boolean {
  return type === "PHOTO";
}

export function hasTranscode(type: string): boolean {
  return type === "VIDEO";
}

/** Поле ручного сканера: значение вводит не человек, а считыватель. */
export function hasScannerSettings(type: string): boolean {
  return type === "SCAN_BARCODE";
}

/**
 * Пропорции кадра у PHOTO. Значение — число-строка: старая админка
 * делит ширину на высоту и кладёт результат, а читает его загрузчик.
 */
export const PHOTO_RATIOS = [
  { value: "1.3", label: "4:3" },
  { value: "1", label: "1:1" },
  { value: "1.5", label: "3:2" },
  { value: "1.7", label: "16:9" },
  { value: "0.7", label: "2:3" },
] as const;

/**
 * Типы, которые можно завести из таблицы.
 *
 * Список короче полного справочника ucode, и это выбор, а не недоделка.
 * Сюда попадает только то, что таблица умеет и показать, и объяснить,
 * и что живо в последнем поколении старой админки (views/views,
 * FormElementGenerator) — справочник `fieldTypes` в ней шире, чем
 * то, что она сама рисует:
 *
 *   не попали  MONEY, PROGRAMMING_LANGUAGE, PRIMARY_KEY — их не рисует
 *              и последнее поколение: тип завёлся бы, а показать его
 *              было бы нечем;
 *              DENTIST — поле, захардкоженное под конкретный проект;
 *              DYNAMIC, LANGUAGE_TYPE, ARRAY — нет ни рендера,
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
      /*
       * Не поле ввода, а подпись-разделитель: показывается название,
       * одинаковое во всех строках (FIELD-AUDIT, F3). В старой админке
       * он так и называется «Text» и лежит среди «Special»; у нас
       * подписан честнее — рядом с «Single line» слово «Text» читалось
       * бы как третий вид текстового поля.
       */
      { type: "TEXT", label: "Heading" },
      { type: "EMAIL", label: "Email" },
      /*
       * PHONE здесь нет намеренно. В справочнике старой админки он
       * закомментирован (`fieldTypes.js:317`) — завести его нельзя
       * с тех пор, как появился INTERNATION_PHONE, и живёт он только
       * у старых полей. Читаются и правятся они по-прежнему: из списка
       * убран только СПОСОБ ЗАВЕСТИ НОВОЕ. Заводить поле без маски
       * и без выбора страны, когда рядом лежит тип с ними, — плодить
       * то, что потом придётся переносить.
       */
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
      { type: "PICK_LIST", label: "Select" },
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
      // Тот же float, но без ограничения знаков после запятой:
      // у количеств и курсов их бывает больше двух.
      { type: "FLOAT_NOLIMIT", label: "Float unlimited" },
      { type: "INCREMENT_ID", label: "Increment id" },
      // Значение выдаёт бэкенд при вставке: случайная строка заданной
      // длины и uuid. Живы в последнем поколении старой админки
      // («Generated string» и «UUID»), в отличие от RANDOM_NUMBERS —
      // тот остался только у полей, заведённых раньше.
      { type: "RANDOM_TEXT", label: "Generated string" },
      { type: "RANDOM_UUID", label: "UUID" },
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
      // Строка по шаблону: слаги в тексте заменяются значениями строки
      // один раз, при вставке (prepareFunctions.go — MANUAL_STRING).
      { type: "MANUAL_STRING", label: "Manual string" },
      { type: "BUTTON", label: "Button" },
      { type: "JSON", label: "JSON" },
      { type: "QR", label: "QR code" },
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
