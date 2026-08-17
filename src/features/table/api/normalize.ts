import {
  STATUS_GROUPS,
  type Field,
  type FieldOption,
  type FieldValidation,
  type Labels,
  type Relation,
  type StatusGroup,
} from "../model/types";
import type { FieldDto, OptionDto, RelationDto } from "./dto";

/**
 * Сырые поля и связи → доменные. Единственное место, где разбираются
 * разные имена одного и того же.
 */

export function toField(dto: FieldDto): Field {
  return {
    id: dto.id ?? "",
    slug: dto.slug ?? "",
    label: pickLabel(dto),
    labels: pickLabels(dto.attributes),
    type: dto.type ?? "",
    // Два имени одного значения: /v2/fields отдаёт relation_field,
    // view_fields внутри связи — relation_id.
    relationId: dto.relation_field || dto.relation_id || null,
    options: toOptions(dto),
    multilanguage: dto.enable_multilanguage === true,
    hasColor: dto.attributes?.has_color === true,
    required: dto.required === true,
    validation: toValidation(dto.attributes),
    editable: isEditable(dto.attributes),
    attributes: dto.attributes ?? {},
    raw: { ...dto },
  };
}

/**
 * Запрет на правку читается строго: право есть, пока его явно не отняли.
 *
 * Иначе ответ, где блока field_permission просто нет (а он приходит
 * не отовсюду), сделал бы всю таблицу нередактируемой — и это выглядело
 * бы не ошибкой, а «так задумано».
 */
function isEditable(attributes: Record<string, unknown> | undefined): boolean {
  if (!attributes) return true;
  if (attributes["disabled"] === true) return false;

  const permission = attributes["field_permission"];
  return !isRecord(permission) || permission["edit_permission"] !== false;
}

/**
 * Проверка ввода: `attributes.validation` — текст регулярного выражения,
 * `attributes.validation_message` — что показать, когда оно не совпало.
 *
 * Три случая означают «не проверять», и все три встречаются в живых
 * данных: ключа нет вовсе, он пуст (форма старой админки записывает
 * `""` каждому полю) и выражение не компилируется — его набирают руками.
 *
 * Старый ucode собирал RegExp безусловно: `new RegExp(field?.attributes
 * ?.validation)`. У поля без настройки это `/undefined/`, то есть правка
 * отклонялась ВСЕГДА, кроме значений со словом «undefined» внутри.
 */
function toValidation(attributes: Record<string, unknown> | undefined): FieldValidation | null {
  const source = attributes?.["validation"];
  if (typeof source !== "string" || !source.trim()) return null;

  const message = attributes?.["validation_message"];

  try {
    return {
      pattern: new RegExp(source),
      message: typeof message === "string" ? message.trim() : "",
    };
  } catch {
    return null;
  }
}

/**
 * У полей-связей колонка `label` содержит не подпись, а служебное имя,
 * которое бэкенд собирает сам: «FROM example_hey TO listings». Человеку
 * его показывать нельзя, и написанное пользователем имя лежит
 * в `attributes.label`. Это единственный тип, где attributes главнее
 * колонки, — потому что колонки с этим значением просто нет.
 */
const GENERATED_LABEL = new Set(["LOOKUP", "LOOKUPS"]);

function pickLabel(dto: FieldDto): string {
  const slug = dto.slug ?? "";

  if (dto.type && GENERATED_LABEL.has(dto.type)) {
    const label = dto.attributes?.["label"];
    // Имени не задали — показываем слаг. Откатываться на колонку нельзя:
    // там лежит то самое «FROM users TO role», ради которого всё это.
    return typeof label === "string" && label.trim() ? label.trim() : slug;
  }

  return dto.label?.trim() || slug;
}

/**
 * Подписи по языкам данных лежат россыпью ключей `label_<short_name>`
 * в одном мешке с обычными настройками. Собираем их в словарь здесь,
 * чтобы компонент не знал ни про префикс, ни про то, что рядом с
 * label_en лежит label_to_en, который к подписи отношения не имеет.
 */
const LABEL_PREFIX = "label_";

export function pickLabels(attributes: Record<string, unknown> | undefined): Labels {
  const labels: Labels = {};
  if (!attributes) return labels;

  for (const [key, value] of Object.entries(attributes)) {
    if (!key.startsWith(LABEL_PREFIX) || typeof value !== "string") continue;

    // label_to_en и label_to_cyr — подпись ОБРАТНОЙ стороны связи.
    const language = key.slice(LABEL_PREFIX.length);
    if (language.startsWith("to_")) continue;

    labels[language] = value;
  }

  return labels;
}

/**
 * Варианты выбора. Два типа хранятся по-разному, и это не вкусовщина
 * бэкенда, а разные экраны настройки:
 *
 *   MULTISELECT  attributes.options[] — {slug, value, label, label_<яз>, color}
 *                В строке лежит slug. `value` при этом занят подписью
 *                на базовом языке, то есть НЕ является значением.
 *
 *   STATUS       attributes.{todo,progress,complete}.options[] — три списка
 *                по стадиям. У варианта есть только value (оно же значение
 *                в строке), color и label_<яз>. Поля label нет вовсе.
 *
 * Приводим к одной форме: ключ поиска всегда `value`.
 */
export function toOptions(dto: FieldDto): Map<string, FieldOption> {
  const options = new Map<string, FieldOption>();
  const attributes = dto.attributes;
  if (!attributes) return options;

  if (dto.type === "MULTISELECT") {
    for (const raw of asOptions(attributes["options"])) {
      // Именно slug: по нему значение и лежит в строке.
      add(options, raw, raw.slug ?? "", null);
    }
    return options;
  }

  if (dto.type === "STATUS") {
    for (const group of STATUS_GROUPS) {
      const bag = attributes[group];
      const list = isRecord(bag) ? asOptions(bag["options"]) : [];
      for (const raw of list) add(options, raw, raw.value ?? "", group);
    }
  }

  return options;
}

function add(
  target: Map<string, FieldOption>,
  raw: OptionDto,
  value: string,
  group: StatusGroup | null,
) {
  if (!value) return;

  target.set(value, {
    value,
    label: typeof raw.label === "string" ? raw.label : "",
    labels: pickLabels(raw as unknown as Record<string, unknown>),
    color: raw.color ?? null,
    icon: raw.icon ?? null,
    group,
  });
}

function asOptions(value: unknown): OptionDto[] {
  return Array.isArray(value) ? (value as OptionDto[]) : [];
}

/**
 * Связь глазами КОНКРЕТНОЙ таблицы.
 *
 * Своя сторона определяется по слагу, а не берётся из table_from:
 * связь двунаправленная и в базе лежит один раз. Для `users` связь
 * `users → client_type` записана как from=users, to=client_type,
 * а та же связь со стороны `client_type` — та же строка, но целевая
 * таблица теперь другая. Брать всегда `table_to` значит для половины
 * связей показывать саму себя.
 */
export function toRelation(dto: RelationDto, tableSlug: string): Relation {
  const from = dto.table_from;
  const to = dto.table_to;

  // Наша сторона — та, чей слаг совпал. Если не совпал ни один, считаем
  // своей from: так вело себя прежнее поведение, и хуже не станет.
  const ours = to?.slug === tableSlug ? to : from;
  const other = ours === from ? to : from;

  /*
   * view_fields приходят объектами полей. Полезны только поля ЧУЖОЙ
   * таблицы: по ним читается связанная строка. Своё поле-связь бэкенд
   * кладёт туда у ненастроенной связи, и по нему в данных не найдётся
   * ничего — показывать «пусто» честнее, чем uuid.
   */
  const theirs = (dto.view_fields ?? []).filter(
    (field) => !ours?.id || field.table_id !== ours.id,
  );

  const viewFieldSlugs = theirs.map((field) => field.slug ?? "").filter(Boolean);
  const viewFieldIds = theirs.map((field) => field.id ?? "").filter(Boolean);

  return {
    id: dto.id ?? "",
    type: dto.type ?? "",
    toSlug: other?.slug ?? "",
    toLabel: other?.label?.trim() || other?.slug || "",
    toLabels: pickLabels(other?.attributes),
    // Колонка-связь лежит в таблице table_from. Если это не мы — в нашей
    // строке её нет, и наружу отдаём пусто, а не чужой слаг.
    fieldFrom: ours === from ? (dto.field_from ?? "") : "",
    viewFieldSlugs,
    viewFieldIds,
    raw: { ...dto },
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}
