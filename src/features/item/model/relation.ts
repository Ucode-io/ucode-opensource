import type { Field } from "@/features/table";
import { relationLabel } from "@/shared/lib/relation-label";
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
  slugs: string[] | undefined,
): { guid: string; label: string }[] {
  if (!slugs?.length) return [];

  const related = row[relationDataKey(field.slug)];

  // Связанная строка бывает и списком: у поля-связи с той стороны,
  // где строк несколько, бэкенд отдаёт массив.
  return (Array.isArray(related) ? related : [related])
    .filter(isRecord)
    .map((item) => ({ guid: String(item["guid"] ?? ""), label: relationLabel(item, slugs) }))
    .filter((item) => item.guid || item.label);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
