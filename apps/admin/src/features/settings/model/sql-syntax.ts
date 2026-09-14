/**
 * Разбор SQL для подсветки и подсказки.
 *
 * Своими руками, а не библиотекой: грамматика здесь ОДНА и заранее
 * известная. Подсветка кода вообще (CodeDialog функций) от библиотеки
 * действительно зависит — там язык любой, от go до vue, — а тут нужен
 * список слов и пять видов лексем, и это тридцать строк против
 * мегабайта редактора кода с воркерами и своей темой.
 *
 * Разбор приблизительный и таким задуман: он красит, а не исполняет.
 * Настоящий SQL разбирает postgres, и его мнение — единственное, которое
 * решает; ошибиться в цвете слова дешевле, чем тащить парсер ради него.
 */

export type SqlTokenKind = "keyword" | "string" | "number" | "comment" | "punct" | "plain";

/** Лексема с её местом в строке: по нему экран ставит подсказку. */
export type SqlToken = { text: string; kind: SqlTokenKind; start: number };

/**
 * Слова, которые красим и подставляем. Отсортированы — на этом держится
 * выбор подсказки: из нескольких подходящих берётся первое, и «первое»
 * должно быть одним и тем же всегда.
 *
 * Список короткий намеренно: сюда попало то, что человек пишет руками.
 * Полный перечень ключевых слов postgres — под тысячу, и девятьсот
 * из них в консоли админки не встретятся ни разу.
 */
const KEYWORDS = [
  "add", "all", "alter", "analyze", "and", "any", "as", "asc", "begin", "between",
  "by", "cascade", "case", "cast", "coalesce", "column", "commit", "constraint",
  "count", "create", "cross", "default", "delete", "desc", "distinct", "drop",
  "else", "end", "exists", "explain", "false", "foreign", "from", "full", "group",
  "having", "ilike", "in", "index", "inner", "insert", "into", "is", "join", "key",
  "left", "like", "limit", "max", "min", "not", "now", "null", "nullif", "offset",
  "on", "or", "order", "outer", "primary", "references", "rename", "returning",
  "right", "rollback", "select", "set", "sum", "table", "then", "to", "true",
  "union", "unique", "update", "using", "values", "view", "when", "where", "with",
];

const KEYWORD_SET = new Set(KEYWORDS);

/**
 * Одним проходом, потому что порядок вариантов и есть правило разбора:
 * `--` раньше минуса (иначе комментарий стал бы двумя операторами),
 * число раньше слова (иначе `2x` распалось бы иначе).
 *
 * Двойные кавычки не разбираются: в postgres это не строка, а имя
 * с сохранением регистра, и красить его как строку было бы враньём.
 *
 * Незакрытые кавычка и блочный комментарий — тоже лексемы (`'?`, `|$`),
 * и это не снисхождение к кривому SQL: пока строку печатают, она
 * незакрыта всегда. Без этого текст внутри кавычек красился бы обычным
 * до последнего символа, а подсказка лезла бы прямо в строковое
 * значение.
 */
const TOKEN =
  /--[^\n]*|\/\*[\s\S]*?(?:\*\/|$)|'(?:[^']|'')*'?|\d+(?:\.\d+)?|[A-Za-z_][A-Za-z0-9_$]*|[*,;().[\]=<>!+\-/%|]+/g;

/**
 * Текст → лексемы. Пробелы и всё неопознанное тоже попадают в список
 * обычным куском: склейка лексем обязана вернуть исходный текст
 * до символа — иначе подложка с подсветкой разъедется с полем ввода,
 * в котором человек печатает.
 */
export function tokenizeSql(sql: string): SqlToken[] {
  const tokens: SqlToken[] = [];
  let last = 0;

  for (const match of sql.matchAll(TOKEN)) {
    const start = match.index ?? 0;
    if (start > last) tokens.push({ text: sql.slice(last, start), kind: "plain", start: last });

    tokens.push({ text: match[0], kind: kindOf(match[0]), start });
    last = start + match[0].length;
  }

  if (last < sql.length) tokens.push({ text: sql.slice(last), kind: "plain", start: last });

  return tokens;
}

function kindOf(text: string): SqlTokenKind {
  if (text.startsWith("--") || text.startsWith("/*")) return "comment";
  if (text.startsWith("'")) return "string";

  const first = text[0] ?? "";
  if (first >= "0" && first <= "9") return "number";
  if (/[A-Za-z_]/.test(first)) return KEYWORD_SET.has(text.toLowerCase()) ? "keyword" : "plain";

  return "punct";
}

/**
 * Именованные параметры — `:id` в тексте запроса.
 *
 * Регулярка НАРОЧНО такая же наивная, как на бэкенде
 * (`storage/postgres/custom_endpoint.go:23` — `:(\w+)`): она должна
 * находить ровно то, что бэкенд подставит, вместе со всеми его
 * промахами. `where t = '12:30'` он тоже посчитает параметром, а в
 * приведении `id::text` увидит параметр `text` — и заменит его на `$1`,
 * сломав запрос. Умная версия здесь означала бы «предупреждение
 * не показали, а сломалось».
 *
 * Зачем это консоли: `exec-query` параметров НЕ подставляет — он шлёт
 * текст в базу как есть, и postgres отвечает синтаксической ошибкой,
 * которую шлюз выбрасывает (см. `api/sql.ts`). То есть запрос
 * с параметром в консоли выглядит как «выполнено, но пусто». Про это
 * и предупреждаем.
 */
export function sqlParams(sql: string): string[] {
  return [...new Set(Array.from(sql.matchAll(/:(\w+)/g), (match) => match[1] ?? ""))];
}

/**
 * После этих слов идёт ИМЯ ТАБЛИЦЫ, а не ключевое слово.
 *
 * На этом держится разделение двух списков: в `from ord` подсказывать
 * `order by` бессмысленно, а в `ord` посреди условия бессмысленно
 * подсказывать таблицу. Слово перед набранным — единственный признак,
 * который для этого нужен, и он же единственный, который можно узнать,
 * не разбирая запрос целиком.
 */
const TABLE_AFTER = new Set(["from", "join", "into", "update", "table"]);

/** Что набирают под курсором и ждут ли здесь имя таблицы. */
function typing(value: string, caret: number): { word: string; expectsTable: boolean } {
  const word = /[A-Za-z_][A-Za-z0-9_$]*$/.exec(value.slice(0, caret))?.[0] ?? "";
  if (!word) return { word: "", expectsTable: false };

  const before = tokenizeSql(value.slice(0, caret - word.length)).filter((token) =>
    token.text.trim(),
  );

  return { word, expectsTable: TABLE_AFTER.has(before.at(-1)?.text.toLowerCase() ?? "") };
}

/**
 * Слово, для которого экрану стоит спросить у сервера список таблиц.
 *
 * Пусто — не спрашивать ничего сверх обычного: список таблиц постраничный
 * и ищет на сервере (`features/table`, `useTables`), поэтому каждое новое
 * слово здесь — это запрос. Отсюда два условия: слово должно стоять
 * в табличном месте и быть не короче двух букв. В остальное время
 * подсказка берётся из списка ключевых слов и сети не касается.
 */
export function sqlTablePrefix(value: string, caret: number): string {
  const { word, expectsTable } = typing(value, caret);
  return expectsTable && word.length >= 2 ? word : "";
}

/**
 * Хвост подсказки: что дописать к недопечатанному слову.
 *
 * Возвращается именно ХВОСТ, а не слово целиком: экран рисует его
 * серым сразу за набранным, а Tab вставляет как есть — ни выделять
 * набранное, ни сравнивать регистр на вставке не нужно.
 *
 * `tables` — слаги, которые сервер нашёл по тому же слову. В таблицном
 * месте берутся только они: подсказать там ключевое слово значит
 * подсказать заведомо не то.
 *
 * Пусто значит «подсказки нет», и это нормальное состояние: подсказка
 * не всплывает ни в строке, ни в комментарии, ни с одной буквы (с одной
 * подойдёт половина списка), ни когда дальше по строке что-то есть —
 * дописывать в середину значило бы закрыть собой чужой текст.
 */
export function sqlGhost(value: string, caret: number, tables: string[] = []): string {
  const { word, expectsTable } = typing(value, caret);
  if (word.length < 2) return "";

  // Дальше по строке пусто — иначе подсказке негде поместиться.
  if (!/^[ \t]*(\n|$)/.test(value.slice(caret))) return "";

  const token = tokenizeSql(value).find((item) => item.start + item.text.length === caret);
  if (!token || (token.kind !== "plain" && token.kind !== "keyword")) return "";

  const lower = word.toLowerCase();

  /*
   * Слаги сортируются здесь, а не приходят готовыми: порядок задаёт
   * сервер, и из двух подходящих «первым» должен оказываться один
   * и тот же — иначе подсказка меняется от запроса к запросу сама.
   */
  const source = expectsTable ? [...tables].sort() : KEYWORDS;
  const match = source.find((name) => name.toLowerCase().startsWith(lower) && name.length > word.length);
  if (!match) return "";

  const tail = match.slice(word.length);

  // Слаг дописывается как есть: это имя в базе, а не слово языка.
  if (expectsTable) return tail;

  // Регистр — по набранному: тот, кто пишет SELECT, ждёт FROM, а не from.
  return word === word.toUpperCase() ? tail.toUpperCase() : tail;
}
