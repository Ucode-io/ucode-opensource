/**
 * Строка таблицы. Форма задаётся схемой, которую админ меняет в
 * конструкторе, поэтому типизировать её полями нельзя — только ключом.
 *
 * `guid` — первичный ключ во всех таблицах ucode (колонка guid,
 * 000001_init_tables.up.sql). Не `id`.
 */
export type Item = Record<string, unknown> & { guid?: string };

/**
 * Значение поля-связи бэкенд кладёт вторым ключом: рядом с `author_id`
 * появляется `author_id_data` со всей связанной строкой.
 */
export const relationDataKey = (fieldSlug: string) => `${fieldSlug}_data`;
