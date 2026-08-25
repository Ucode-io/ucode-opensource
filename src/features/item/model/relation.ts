import type { Field, Relation } from "@/features/table";
import { relationLabel, type ViewField } from "@/shared/lib/relation-label";
import { relationDataKey, type Item } from "./types";

// Подпись связанной строки общая с настройками агрегата — см. shared/lib.
export { relationLabel };

/**
 * Правка связи из таблицы.
 *
 * Связь ровно одна: Many2One. Это не упрощение с нашей стороны —
 * так устроен и актуальный конструктор ucode: форма новой связи
 * (RelationFieldForm) спрашивает только целевую таблицу и поля показа,
 * выбора типа в ней нет вовсе. Many2Many и One2Many остались в старом
 * экране настроек и в базе, но заводить их из таблицы нельзя.
 *
 * Отсюда и правило: ссылка лежит колонкой в самой строке, и связать —
 * это обычная правка поля. Всё остальное открывается только на чтение:
 * у Many2Many связи лежат в третьей таблице, у One2Many — в ЧУЖИХ
 * строках, и «связать» там значит незаметно переписать чужую ячейку.
 */
export function isLinkable(type: string): boolean {
  return type.toLowerCase() === "many2one";
}

export type RelationEdit = {
  /** Колонка со ссылкой в строке. */
  fieldSlug: string;
  /** Строка, у которой правим связь. */
  rowGuid: string;
  /** null — снять связь. */
  itemGuid: string | null;
};

/**
 * Что показывать вместо uuid: поля связанной строки, выбранные
 * в настройках связи (view_fields). Связанная строка приходит рядом
 * со значением: author_id → author_id_data.
 *
 * Ничего не настроено — показывать нечего. Подставлять `title` или
 * `name` нельзя: это угадывание, а не чтение (см. CONTEXT, Relation).
 */
export function relationSelection(
  row: Item,
  field: Field,
  fields: ViewField[] | undefined,
  /** Язык ДАННЫХ: им отбирается колонка мультиязычного поля показа. */
  language = "",
): { guid: string; label: string }[] {
  if (!fields?.length) return [];

  const related = row[relationDataKey(field.slug)];

  // Связанная строка бывает и списком: у поля-связи с той стороны,
  // где строк несколько, бэкенд отдаёт массив.
  return (Array.isArray(related) ? related : [related])
    .filter(isRecord)
    .map((item) => ({
      guid: String(item["guid"] ?? ""),
      label: relationLabel(item, fields, language),
    }))
    .filter((item) => item.guid || item.label);
}

/**
 * Автофильтр связи: какие строки чужой таблицы вообще можно выбрать.
 *
 * Настройка лежит в самой связи — `auto_filters` списком пар
 * `{field_from, field_to}`, где `field_from` это слаг поля В ЭТОЙ строке,
 * а `field_to` — слаг поля в ЧУЖОЙ таблице. Так делается зависимый
 * выбор: город отбирается по региону, выбранному строкой выше.
 *
 * Уходит плоскими ключами в тело `get-list`, рядом с обычным отбором —
 * туда же их кладёт и старая админка (`RelationField.jsx`,
 * `autoFiltersValue`). Ключ `auto_filter`, который бэкенд тоже понимает
 * (`storage/postgres/build_query.go:266`), не нужен: он склеивает
 * условия через OR, а пары независимы и должны действовать вместе.
 *
 * Поле-источник ещё не заполнено — условия нет вовсе: пока регион
 * не выбран, показываются все города, а не ни одного.
 */
export function autoFilterValues(relation: Relation, row: Item): Record<string, unknown> {
  const pairs = relation.raw["auto_filters"];
  if (!Array.isArray(pairs)) return {};

  const values: Record<string, unknown> = {};

  for (const pair of pairs) {
    if (!isRecord(pair)) continue;

    const from = String(pair["field_from"] ?? "");
    const to = String(pair["field_to"] ?? "");
    if (!from || !to) continue;

    const value = row[from];
    if (value === undefined || value === null || value === "") continue;

    values[to] = value;
  }

  return values;
}

/** Кто заводит запись. Из сеанса, а не из схемы: это про человека. */
export type SelfIdentity = {
  /** Id вошедшего — строка в таблице входа проекта. */
  userId: string;
  /** «Своя строка» в чужих таблицах: слаг → guid (см. session.getObjectIds). */
  objectIds: Record<string, string>;
};

/**
 * Значения по умолчанию, которых поле о себе не знает.
 *
 * Связь можно пометить «подставлять своего» — тогда у НОВОЙ записи
 * колонка-ссылка заполняется тем, кто её заводит: «Автор», «Ответственный»,
 * «Мой филиал». Настройка живёт на связи (`Relation.selfDefault`), а
 * значение — в сеансе, поэтому собирается это здесь, а не в blankItem:
 * тот про поля и чист.
 *
 * Только исходящие связи: у входящей колонки-ссылки в нашей строке нет
 * вовсе — идентификатор лежит в чужой.
 */
export function selfDefaults(relations: Relation[], me: SelfIdentity): Item {
  const row: Item = {};

  for (const relation of relations) {
    if (!relation.selfDefault || relation.direction !== "outgoing") continue;
    if (!relation.linkField) continue;

    const value =
      relation.selfDefault === "user" ? me.userId : me.objectIds[relation.toSlug];

    // Нет значения — нет и подстановки: пустая строка в колонке-ссылке
    // это не «никто», а битый идентификатор.
    if (value) row[relation.linkField] = value;
  }

  return row;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
