/**
 * Название → слаг.
 *
 * Слаг поля становится ИМЕНЕМ КОЛОНКИ в SQL: бэкенд подставляет его
 * в `ALTER TABLE ... ADD COLUMN <слаг> <тип>` без кавычек. Отсюда все
 * правила ниже — это не косметика, а то, что отделяет работающее поле
 * от ответа 500 с текстом про синтаксис.
 *
 * Кириллица не выбрасывается, а транслитерируется: названия в проектах
 * русские и узбекские, и «Название» без транслитерации дало бы пустой
 * слаг — то есть поле, которое нельзя создать.
 */

const CYRILLIC: Record<string, string> = {
  а: "a", б: "b", в: "v", г: "g", д: "d", е: "e", ё: "e", ж: "zh",
  з: "z", и: "i", й: "y", к: "k", л: "l", м: "m", н: "n", о: "o",
  п: "p", р: "r", с: "s", т: "t", у: "u", ф: "f", х: "h", ц: "c",
  ч: "ch", ш: "sh", щ: "sch", ъ: "", ы: "y", ь: "", э: "e", ю: "yu",
  я: "ya",
  // Узбекская кириллица: этих букв в русской раскладке нет.
  ў: "o", қ: "q", ғ: "g", ҳ: "h",
};

/**
 * Слова, которые postgres разберёт как часть команды, а не как имя
 * колонки. Список короткий сознательно: сюда попадает то, что человек
 * реально назовёт полем («Порядок» → order, «Группа» → group), а не
 * весь словарь стандарта.
 */
const RESERVED = new Set([
  "all", "and", "any", "as", "asc", "between", "by", "case", "check",
  "column", "constraint", "create", "default", "desc", "distinct", "do",
  "else", "end", "false", "for", "from", "group", "having", "in", "index",
  "is", "join", "key", "left", "like", "limit", "not", "null", "offset",
  "on", "or", "order", "primary", "references", "right", "select", "table",
  "then", "to", "true", "union", "unique", "user", "using", "when", "where",
]);

export function slugify(value: string): string {
  const latin = value
    .toLowerCase()
    .split("")
    .map((char) => CYRILLIC[char] ?? char)
    .join("");

  const slug = latin
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/_+/g, "_")
    .replace(/^_|_$/g, "");

  if (!slug) return "";

  // Имя колонки не начинается с цифры, а зарезервированное слово
  // ломает саму команду. И то и другое чинится одним символом.
  if (/^[0-9]/.test(slug)) return `f_${slug}`;
  return RESERVED.has(slug) ? `${slug}_` : slug;
}

/** Годится ли слаг как имя колонки. Проверяется и то, что ввели руками. */
export function isValidSlug(value: string): boolean {
  return /^[a-z][a-z0-9_]*$/.test(value) && !RESERVED.has(value);
}
